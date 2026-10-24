// Package goport opens public GoPort tunnels to local TCP ports.
package goport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	neturl "net/url"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/yamux"
)

const (
	// DefaultServer is the public GoPort TCP tunnel endpoint.
	DefaultServer = "goport.uz:7000"

	defaultDialTimeout  = 10 * time.Second
	defaultReconnectMin = time.Second
	defaultReconnectMax = 30 * time.Second
	rateLimitedDelay    = time.Minute
)

// permanentRefusals are server refusals that retrying cannot fix; the tunnel
// stops reconnecting when it gets one. Kept in step with the CLI.
var permanentRefusals = []string{
	"authentication required",
	"available on pro",
	"traffic limit",
	"outside your plan allowance",
	"cannot be used together",
	"subdomain must",
	"subdomain cannot",
	"subdomain can only",
	"is reserved",
	"missing port",
	"belongs to another account",
	"stopped from the dashboard",
}

// RefusalError is the server refusing a tunnel, as opposed to the network
// failing on the way.
type RefusalError struct {
	Message string
}

func (e *RefusalError) Error() string { return "goport: " + e.Message }

// Permanent reports whether retrying cannot help (bad token, plan, name, or
// the tunnel was stopped from the dashboard).
func (e *RefusalError) Permanent() bool {
	msg := strings.ToLower(e.Message)
	for _, fragment := range permanentRefusals {
		if strings.Contains(msg, fragment) {
			return true
		}
	}
	return false
}

func (e *RefusalError) rateLimited() bool {
	return strings.Contains(strings.ToLower(e.Message), "too many tunnel registrations")
}

// Tunnel is an active public tunnel to a local port.
type Tunnel interface {
	URL() string
	Close() error
}

// Option configures a Client returned by New.
type Option func(*Client)

// WithServer overrides the default GoPort TCP tunnel server address.
func WithServer(addr string) Option {
	return func(c *Client) {
		c.serverAddr = strings.TrimSpace(addr)
	}
}

// WithLogOutput sends the tunnel's internal (yamux) log lines to w. By
// default they are discarded so nothing lands on the host app's stderr.
func WithLogOutput(w io.Writer) Option {
	return func(c *Client) {
		if w != nil {
			c.logOutput = w
		}
	}
}

// WithStopHandler sets fn to be called, once and on its own goroutine, when
// the tunnel stops reconnecting because the server refused it for good (for
// example, it was stopped from the dashboard). err is a *RefusalError.
func WithStopHandler(fn func(err error)) Option {
	return func(c *Client) {
		c.onStop = fn
	}
}

// Client is a small builder for opening GoPort tunnels.
type Client struct {
	token        string
	serverAddr   string
	subdomain    string
	reset        bool
	dialTimeout  time.Duration
	reconnectMin time.Duration
	reconnectMax time.Duration
	logOutput    io.Writer
	onStop       func(error)
}

// New returns a tunnel client builder authenticated with token.
func New(token string, opts ...Option) *Client {
	c := &Client{
		token:        strings.TrimSpace(token),
		serverAddr:   DefaultServer,
		dialTimeout:  defaultDialTimeout,
		reconnectMin: defaultReconnectMin,
		reconnectMax: defaultReconnectMax,
		logOutput:    io.Discard,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// WithServer overrides the default GoPort TCP tunnel server address.
func (c *Client) WithServer(addr string) *Client {
	c.serverAddr = strings.TrimSpace(addr)
	return c
}

// Subdomain requests a custom public subdomain for the tunnel.
func (c *Client) Subdomain(name string) *Client {
	c.subdomain = strings.TrimSpace(name)
	c.reset = false
	return c
}

// Reset requests a fresh random subdomain for the tunnel.
func (c *Client) Reset() *Client {
	c.subdomain = ""
	c.reset = true
	return c
}

// Open establishes a tunnel to localhost:port.
func (c *Client) Open(port int) (Tunnel, error) {
	cfg := c.config()
	if cfg.token == "" {
		return nil, errors.New("goport: token is required")
	}
	if cfg.serverAddr == "" {
		return nil, errors.New("goport: server address is required")
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("goport: invalid port %d", port)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t := &clientTunnel{
		cfg:    cfg,
		port:   port,
		ctx:    ctx,
		cancel: cancel,
	}
	if err := t.connect(true); err != nil {
		cancel()
		return nil, err
	}

	t.runWG.Add(1)
	go t.run()

	return t, nil
}

func (c *Client) config() clientConfig {
	return clientConfig{
		token:        c.token,
		serverAddr:   c.serverAddr,
		subdomain:    c.subdomain,
		reset:        c.reset,
		dialTimeout:  c.dialTimeout,
		reconnectMin: c.reconnectMin,
		reconnectMax: c.reconnectMax,
		logOutput:    c.logOutput,
		onStop:       c.onStop,
	}
}

type clientConfig struct {
	token        string
	serverAddr   string
	subdomain    string
	reset        bool
	dialTimeout  time.Duration
	reconnectMin time.Duration
	reconnectMax time.Duration
	logOutput    io.Writer
	onStop       func(error)
}

type registrationRequest struct {
	Type      string `json:"type"`
	Port      string `json:"port"`
	Subdomain string `json:"subdomain,omitempty"`
	Reset     bool   `json:"reset,omitempty"`
	Token     string `json:"token,omitempty"`
	// Resume asks for the subdomain held before the connection dropped. It
	// is not a custom-name request, so it works on the free plan too.
	Resume string `json:"resume,omitempty"`
}

// sessionConfig matches the CLI: keepalives catch dead connections, a long
// write timeout tolerates slow uplinks, and yamux logs go to logOutput rather
// than the host app's stderr.
func sessionConfig(logOutput io.Writer) *yamux.Config {
	cfg := yamux.DefaultConfig()
	cfg.EnableKeepAlive = true
	cfg.KeepAliveInterval = 20 * time.Second
	cfg.ConnectionWriteTimeout = 30 * time.Second
	if logOutput == nil {
		logOutput = io.Discard
	}
	cfg.LogOutput = logOutput
	return cfg
}

type registrationResponse struct {
	Subdomain string `json:"subdomain"`
	URL       string `json:"url"`
	Error     string `json:"error,omitempty"`
}

type clientTunnel struct {
	cfg  clientConfig
	port int

	ctx    context.Context
	cancel context.CancelFunc

	runWG      sync.WaitGroup
	handlersWG sync.WaitGroup
	closeOnce  sync.Once

	mu                sync.RWMutex
	conn              net.Conn
	session           *yamux.Session
	url               string
	assignedSubdomain string
	// preferV6 is set once the local app answered on [::1] but not 127.0.0.1.
	preferV6 bool
}

func (t *clientTunnel) URL() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.url
}

func (t *clientTunnel) Close() error {
	t.closeOnce.Do(func() {
		t.cancel()
		t.closeCurrent()
		t.runWG.Wait()
		t.handlersWG.Wait()
	})
	return nil
}

func (t *clientTunnel) run() {
	defer t.runWG.Done()

	backoff := t.cfg.reconnectMin
	if backoff <= 0 {
		backoff = defaultReconnectMin
	}

	for t.ctx.Err() == nil {
		_ = t.serve()
		if t.ctx.Err() != nil {
			return
		}
		t.closeCurrent()

		if !t.wait(backoff) {
			return
		}

		for t.ctx.Err() == nil {
			err := t.connect(false)
			if err == nil {
				backoff = t.cfg.reconnectMin
				if backoff <= 0 {
					backoff = defaultReconnectMin
				}
				break
			}
			wait := backoff
			var refused *RefusalError
			if errors.As(err, &refused) {
				if refused.Permanent() {
					if t.cfg.onStop != nil {
						go t.cfg.onStop(err)
					}
					return
				}
				if refused.rateLimited() {
					wait = max(wait, rateLimitedDelay)
				}
			}
			if !t.wait(wait) {
				return
			}
			backoff = nextBackoff(backoff, t.cfg.reconnectMax)
		}
	}
}

func (t *clientTunnel) serve() error {
	session := t.currentSession()
	if session == nil {
		return errors.New("goport: tunnel session is not connected")
	}

	for {
		stream, err := session.Accept()
		if err != nil {
			return err
		}

		t.handlersWG.Add(1)
		go func() {
			defer t.handlersWG.Done()
			t.handleStream(stream)
		}()
	}
}

func (t *clientTunnel) currentSession() *yamux.Session {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.session
}

func (t *clientTunnel) connect(initial bool) error {
	dialer := net.Dialer{
		Timeout:   t.cfg.dialTimeout,
		KeepAlive: 30 * time.Second,
	}

	conn, err := dialer.DialContext(t.ctx, "tcp", t.cfg.serverAddr)
	if err != nil {
		return fmt.Errorf("goport: connect to %s: %w", t.cfg.serverAddr, err)
	}

	if t.cfg.dialTimeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(t.cfg.dialTimeout))
	}

	req := registrationRequest{
		Type:      "http",
		Port:      strconv.Itoa(t.port),
		Token:     t.cfg.token,
		Subdomain: t.cfg.subdomain,
	}
	if initial {
		req.Reset = t.cfg.reset
	} else {
		// The assigned name goes in Resume: as Subdomain the server treats it
		// as a custom-name request, which free accounts can't make.
		req.Resume = t.reconnectSubdomain()
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		_ = conn.Close()
		return fmt.Errorf("goport: send handshake: %w", err)
	}

	// The reply is one JSON line, and yamux frames can follow in the same
	// read. Read exactly that line and hand the rest of the buffer to yamux.
	reader := bufio.NewReader(conn)
	line, err := reader.ReadSlice('\n')
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("goport: read handshake: %w", err)
	}
	var resp registrationResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		_ = conn.Close()
		return fmt.Errorf("goport: read handshake: %w", err)
	}
	if resp.Error != "" {
		_ = conn.Close()
		return &RefusalError{Message: resp.Error}
	}

	publicURL := responseURL(resp)
	if publicURL == "" {
		_ = conn.Close()
		return errors.New("goport: server returned empty tunnel url")
	}

	_ = conn.SetDeadline(time.Time{})

	session, err := yamux.Client(&bufferedConn{Conn: conn, r: reader}, sessionConfig(t.cfg.logOutput))
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("goport: start yamux client: %w", err)
	}

	t.mu.Lock()
	if t.ctx.Err() != nil {
		t.mu.Unlock()
		_ = session.Close()
		_ = conn.Close()
		return t.ctx.Err()
	}
	t.conn = conn
	t.session = session
	t.url = publicURL
	if subdomain := responseSubdomain(resp); subdomain != "" {
		t.assignedSubdomain = subdomain
	}
	t.mu.Unlock()

	return nil
}

// bufferedConn reads through the handshake's reader, so bytes it already
// buffered past the reply reach yamux first.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

func (t *clientTunnel) reconnectSubdomain() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.assignedSubdomain != "" {
		return t.assignedSubdomain
	}
	return t.cfg.subdomain
}

func (t *clientTunnel) closeCurrent() {
	t.mu.Lock()
	session := t.session
	conn := t.conn
	t.session = nil
	t.conn = nil
	t.mu.Unlock()

	if session != nil {
		_ = session.Close()
	}
	if conn != nil {
		_ = conn.Close()
	}
}

func (t *clientTunnel) wait(d time.Duration) bool {
	if d <= 0 {
		d = defaultReconnectMin
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-t.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (t *clientTunnel) handleStream(stream net.Conn) {
	defer stream.Close()

	localConn, err := t.dialLocal()
	if err != nil {
		writeBadGateway(stream)
		return
	}
	defer localConn.Close()

	proxy(stream, localConn)
}

// dialLocal connects to the local app on 127.0.0.1, or on [::1] when that
// refuses (some dev servers listen on IPv6 loopback only). The address that
// answered is tried first next time.
func (t *clientTunnel) dialLocal() (net.Conn, error) {
	port := strconv.Itoa(t.port)
	addrs := []string{net.JoinHostPort("127.0.0.1", port), net.JoinHostPort("::1", port)}
	t.mu.RLock()
	if t.preferV6 {
		addrs[0], addrs[1] = addrs[1], addrs[0]
	}
	t.mu.RUnlock()

	dialer := net.Dialer{Timeout: t.cfg.dialTimeout}
	conn, err := dialer.DialContext(t.ctx, "tcp", addrs[0])
	if err == nil || !isConnRefused(err) {
		return conn, err
	}
	conn, otherErr := dialer.DialContext(t.ctx, "tcp", addrs[1])
	if otherErr != nil {
		return nil, err
	}
	t.mu.Lock()
	t.preferV6 = strings.HasPrefix(addrs[1], "[")
	t.mu.Unlock()
	return conn, nil
}

// isConnRefused also matches WSAECONNREFUSED, which Go does not map to
// syscall.ECONNREFUSED on Windows.
func isConnRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.Errno(10061))
}

func proxy(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go copyAndClose(a, b, done)
	go copyAndClose(b, a, done)

	<-done
	_ = a.Close()
	_ = b.Close()
	<-done
}

func copyAndClose(dst, src net.Conn, done chan<- struct{}) {
	_, _ = io.Copy(dst, src)
	closeWrite(dst)
	done <- struct{}{}
}

func closeWrite(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = c.Close()
}

// writeBadGateway answers a visitor when the local app can't be reached. The
// local port and the OS error are not shown to visitors.
func writeBadGateway(w io.Writer) {
	const body = "The local app didn't respond.\n"
	_, _ = fmt.Fprintf(w, "HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
}

func responseURL(resp registrationResponse) string {
	if url := strings.TrimSpace(resp.URL); url != "" {
		return url
	}

	subdomain := strings.TrimSpace(resp.Subdomain)
	if strings.HasPrefix(subdomain, "https://") || strings.HasPrefix(subdomain, "http://") {
		return subdomain
	}
	return ""
}

func responseSubdomain(resp registrationResponse) string {
	subdomain := strings.TrimSpace(resp.Subdomain)
	if subdomain == "" {
		return subdomainFromURL(resp.URL)
	}
	if strings.HasPrefix(subdomain, "https://") || strings.HasPrefix(subdomain, "http://") {
		return subdomainFromURL(subdomain)
	}
	return subdomain
}

func subdomainFromURL(raw string) string {
	parsed, err := neturl.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	host := parsed.Hostname()
	if i := strings.IndexByte(host, '.'); i > 0 {
		return host[:i]
	}
	return host
}

func nextBackoff(current, max time.Duration) time.Duration {
	if current <= 0 {
		current = defaultReconnectMin
	}
	if max <= 0 {
		max = defaultReconnectMax
	}

	next := current * 2
	if next > max {
		return max
	}
	return next
}
