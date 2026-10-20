package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// Result is one run of every phase against one target. It is written as JSON
// and read back by the report command, so field names are part of the format.
type Result struct {
	Tool          string    `json:"tool"`
	Target        string    `json:"target"`
	Round         int       `json:"round"`
	ClientVersion string    `json:"client_version"`
	Region        string    `json:"region,omitempty"`
	Notes         string    `json:"notes,omitempty"`
	BaseHost      string    `json:"base_host"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	Env           Env       `json:"env"`
	Settings      Settings  `json:"settings"`

	Probes      []Probe     `json:"probes"`
	Latency     Latency     `json:"latency"`
	Concurrency Concurrency `json:"concurrency"`
	Downloads   []Transfer  `json:"downloads"`
	Uploads     []Transfer  `json:"uploads"`
	WebSocket   WSResult    `json:"websocket"`
}

type Env struct {
	OS        string `json:"os"`
	OSVersion string `json:"os_version,omitempty"`
	Arch      string `json:"arch"`
	CPU       string `json:"cpu"`
	NumCPU    int    `json:"num_cpu"`
	GoVersion string `json:"go_version"`
}

type Settings struct {
	LatencyRequests     int      `json:"latency_requests"`
	Warmup              int      `json:"warmup"`
	ConcurrencyWorkers  int      `json:"concurrency_workers"`
	ConcurrencyRequests int      `json:"concurrency_requests"`
	ConcurrencyMaxSec   float64  `json:"concurrency_max_sec"`
	DownloadBytes       int64    `json:"download_bytes"`
	Downloads           int      `json:"downloads"`
	UploadBytes         int64    `json:"upload_bytes"`
	Uploads             int      `json:"uploads"`
	WSMessages          int      `json:"ws_messages"`
	WSMessageBytes      int      `json:"ws_message_bytes"`
	ExtraHeaderNames    []string `json:"extra_header_names,omitempty"`
	HostOverride        string   `json:"host_override,omitempty"`
}

// Stats are in milliseconds.
type Stats struct {
	Count  int     `json:"count"`
	Min    float64 `json:"min_ms"`
	Median float64 `json:"median_ms"`
	Mean   float64 `json:"mean_ms"`
	P95    float64 `json:"p95_ms"`
	P99    float64 `json:"p99_ms"`
	Max    float64 `json:"max_ms"`
}

type ErrorCount struct {
	Error string `json:"error"`
	Count int    `json:"count"`
}

type Probe struct {
	Addr     string  `json:"addr"`
	Attempts int     `json:"attempts"`
	OK       int     `json:"ok"`
	MinMs    float64 `json:"min_ms,omitempty"`
	MedianMs float64 `json:"median_ms,omitempty"`
	Error    string  `json:"error,omitempty"`
}

type Latency struct {
	Requests int          `json:"requests"`
	OK       int          `json:"ok"`
	Errors   int          `json:"errors"`
	ErrorLog []ErrorCount `json:"error_log,omitempty"`
	ColdMs   float64      `json:"cold_ms,omitempty"`
	Protocol string       `json:"protocol,omitempty"`
	Stats    *Stats       `json:"stats,omitempty"`
}

type Concurrency struct {
	Workers     int          `json:"workers"`
	Requests    int          `json:"requests"`
	Sent        int          `json:"sent"`
	TimeCapped  bool         `json:"time_capped,omitempty"`
	OK          int          `json:"ok"`
	Errors      int          `json:"errors"`
	ErrorLog    []ErrorCount `json:"error_log,omitempty"`
	DurationSec float64      `json:"duration_sec"`
	ReqPerSec   float64      `json:"req_per_sec"`
	Stats       *Stats       `json:"stats,omitempty"`
}

type Transfer struct {
	Bytes int64 `json:"bytes"`
	OK    bool  `json:"ok"`
	// Set when a transfer failed part-way (usually the transfer timeout):
	// how far it got and the rate up to that point.
	Received    int64   `json:"received_bytes,omitempty"`
	PartialMBps float64 `json:"partial_mb_per_sec,omitempty"`
	TTFBMs      float64 `json:"ttfb_ms,omitempty"`
	Seconds     float64 `json:"seconds,omitempty"`
	MBps        float64 `json:"mb_per_sec,omitempty"`
	Error       string  `json:"error,omitempty"`
}

type WSResult struct {
	Messages     int          `json:"messages"`
	MessageBytes int          `json:"message_bytes"`
	Echoed       int          `json:"echoed"`
	Failures     int          `json:"failures"`
	Reconnects   int          `json:"reconnects"`
	StayedUp     bool         `json:"stayed_up"`
	ConnectMs    float64      `json:"connect_ms,omitempty"`
	ErrorLog     []ErrorCount `json:"error_log,omitempty"`
	Stats        *Stats       `json:"stats,omitempty"`
}

type headerList []string

func (h *headerList) String() string     { return strings.Join(*h, ", ") }
func (h *headerList) Set(v string) error { *h = append(*h, v); return nil }

type bench struct {
	base        *url.URL // what requests are sent to (Host header included)
	dialAddr    string   // where connections actually go
	headers     http.Header
	client      *http.Client // keep-alive client, HTTP/2 when offered
	wsClient    *http.Client // HTTP/1.1 only, no timeout (websocket uses ctx)
	timeout     time.Duration
	xferTimeout time.Duration
}

func runBench(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	rawURL := fs.String("url", "", "base URL of the tunnel (or origin), e.g. https://abc.goport.uz")
	target := fs.String("target", "", "target name recorded in the result, e.g. goport-hosted")
	out := fs.String("out", "", "write the JSON result here (default: stdout)")
	round := fs.Int("round", 1, "round number recorded in the result")
	clientVersion := fs.String("client-version", "", "tunnel client version string to record")
	region := fs.String("region", "", "server/edge region to record, if known")
	notes := fs.String("notes", "", "free-form note recorded in the result")
	hostOverride := fs.String("host", "", "send this Host header (connect to -url's address, route by Host)")
	cpu := fs.String("cpu", "", "CPU model to record (auto-detected when empty)")
	setupErr := fs.String("setup-error", "", "record a run where the tunnel never came up, with this reason")
	var headers headerList
	fs.Var(&headers, "header", "extra request header 'Name: value' (repeatable)")
	var probes headerList
	fs.Var(&probes, "probe", "extra host:port to time TCP connects to (repeatable)")

	nLatency := fs.Int("latency-requests", 200, "sequential GET /ping requests")
	warmup := fs.Int("warmup", 5, "uncounted requests before the latency phase")
	workers := fs.Int("concurrency", 10, "concurrent workers")
	nConc := fs.Int("concurrency-requests", 1000, "total GET /ping requests in the concurrency phase")
	concMax := fs.Duration("concurrency-max", 90*time.Second, "stop starting new concurrency requests after this long")
	dlBytes := fs.Int64("download-bytes", 10_000_000, "bytes per download")
	nDL := fs.Int("downloads", 3, "number of downloads")
	ulBytes := fs.Int64("upload-bytes", 10_000_000, "bytes per upload")
	nUL := fs.Int("uploads", 1, "number of uploads")
	wsMsgs := fs.Int("ws-messages", 100, "WebSocket echo round trips")
	wsSize := fs.Int("ws-size", 1024, "WebSocket message size in bytes")
	timeout := fs.Duration("timeout", 30*time.Second, "timeout for small requests")
	xferTimeout := fs.Duration("transfer-timeout", 120*time.Second, "timeout for each large transfer; partial progress is recorded")
	_ = fs.Parse(args)

	if *target == "" || (*rawURL == "" && *setupErr == "") {
		return errors.New("-url and -target are required")
	}
	if *rawURL == "" {
		*rawURL = "http://unavailable.invalid"
	}
	if *workers < 1 || *workers > 64 {
		return errors.New("-concurrency must be between 1 and 64")
	}

	b, err := newBench(*rawURL, *hostOverride, headers, *timeout, *xferTimeout)
	if err != nil {
		return err
	}

	res := &Result{
		Tool:          userAgent,
		Target:        *target,
		Round:         *round,
		ClientVersion: *clientVersion,
		Region:        *region,
		Notes:         *notes,
		BaseHost:      redactHost(b.base.Host),
		StartedAt:     time.Now().UTC(),
		Env:           detectEnv(*cpu),
		Settings: Settings{
			LatencyRequests:     *nLatency,
			Warmup:              *warmup,
			ConcurrencyWorkers:  *workers,
			ConcurrencyRequests: *nConc,
			ConcurrencyMaxSec:   concMax.Seconds(),
			DownloadBytes:       *dlBytes,
			Downloads:           *nDL,
			UploadBytes:         *ulBytes,
			Uploads:             *nUL,
			WSMessages:          *wsMsgs,
			WSMessageBytes:      *wsSize,
			HostOverride:        *hostOverride,
		},
	}
	for name := range b.headers {
		res.Settings.ExtraHeaderNames = append(res.Settings.ExtraHeaderNames, name)
	}
	sort.Strings(res.Settings.ExtraHeaderNames)

	logf := func(format string, a ...any) {
		fmt.Fprintf(os.Stderr, "[%s r%d] %s\n", *target, *round, fmt.Sprintf(format, a...))
	}

	if *setupErr != "" {
		recordSetupFailure(res, *setupErr)
		logf("setup failed: %s", *setupErr)
		return writeResult(res, *out)
	}

	probeAddrs := append([]string{b.dialAddr}, probes...)
	for _, addr := range probeAddrs {
		p := probeTCP(addr, 5)
		res.Probes = append(res.Probes, p)
		logf("tcp connect %s: median %.1f ms (%d/%d ok) %s", addr, p.MedianMs, p.OK, p.Attempts, p.Error)
	}

	res.Latency = b.latency(*nLatency, *warmup)
	logf("latency: %d/%d ok, median %s", res.Latency.OK, res.Latency.Requests, fmtStat(res.Latency.Stats, "median"))

	res.Concurrency = b.concurrency(*workers, *nConc, *concMax)
	logf("concurrency: %d/%d ok, %.1f req/s", res.Concurrency.OK, res.Concurrency.Requests, res.Concurrency.ReqPerSec)

	for i := 0; i < *nDL; i++ {
		t := b.download(*dlBytes)
		res.Downloads = append(res.Downloads, t)
		logf("download %d: ok=%v %.2f MB/s %s", i+1, t.OK, t.MBps, t.Error)
	}
	for i := 0; i < *nUL; i++ {
		t := b.upload(*ulBytes)
		res.Uploads = append(res.Uploads, t)
		logf("upload %d: ok=%v %.2f MB/s %s", i+1, t.OK, t.MBps, t.Error)
	}

	res.WebSocket = b.websocket(*wsMsgs, *wsSize)
	logf("websocket: %d/%d echoed, stayed up=%v, median %s", res.WebSocket.Echoed, res.WebSocket.Messages, res.WebSocket.StayedUp, fmtStat(res.WebSocket.Stats, "median"))

	return writeResult(res, *out)
}

// recordSetupFailure fills every phase with failures, so a tunnel that never
// came up counts against its target instead of silently missing a round.
func recordSetupFailure(res *Result, reason string) {
	s := res.Settings
	msg := "setup failed: " + reason
	res.BaseHost = ""
	res.Notes = strings.TrimSpace(res.Notes + " Setup failed: " + reason)
	res.Latency = Latency{Requests: s.LatencyRequests, Errors: s.LatencyRequests, ErrorLog: []ErrorCount{{Error: msg, Count: s.LatencyRequests}}}
	res.Concurrency = Concurrency{Workers: s.ConcurrencyWorkers, Requests: s.ConcurrencyRequests, Errors: s.ConcurrencyRequests, ErrorLog: []ErrorCount{{Error: msg, Count: s.ConcurrencyRequests}}}
	for i := 0; i < s.Downloads; i++ {
		res.Downloads = append(res.Downloads, Transfer{Bytes: s.DownloadBytes, Error: msg})
	}
	for i := 0; i < s.Uploads; i++ {
		res.Uploads = append(res.Uploads, Transfer{Bytes: s.UploadBytes, Error: msg})
	}
	res.WebSocket = WSResult{Messages: s.WSMessages, MessageBytes: s.WSMessageBytes, Failures: s.WSMessages, ErrorLog: []ErrorCount{{Error: msg, Count: 1}}}
}

func writeResult(res *Result, out string) error {
	res.FinishedAt = time.Now().UTC()
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if out == "" {
		_, err = os.Stdout.Write(data)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, data, 0o644)
}

func fmtStat(s *Stats, which string) string {
	if s == nil {
		return "n/a"
	}
	switch which {
	case "median":
		return fmt.Sprintf("%.1f ms", s.Median)
	}
	return ""
}

func newBench(rawURL, hostOverride string, headerArgs []string, timeout, xferTimeout time.Duration) (*bench, error) {
	base, err := url.Parse(strings.TrimRight(rawURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("bad -url: %w", err)
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return nil, fmt.Errorf("-url must start with http:// or https://")
	}
	port := base.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[base.Scheme]
	}
	dialAddr := net.JoinHostPort(base.Hostname(), port)

	if hostOverride != "" {
		h := hostOverride
		if base.Port() != "" {
			h = net.JoinHostPort(hostOverride, base.Port())
		}
		base.Host = h
	}

	hdr := http.Header{}
	for _, h := range headerArgs {
		name, value, ok := strings.Cut(h, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("bad -header %q, want 'Name: value'", h)
		}
		hdr.Add(strings.TrimSpace(name), strings.TrimSpace(value))
	}

	dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	dial := func(ctx context.Context, network, _ string) (net.Conn, error) {
		// Always connect to the -url address, even when the Host header
		// names something else (routing by Host on a loopback server).
		return dialer.DialContext(ctx, network, dialAddr)
	}

	transport := &http.Transport{
		DialContext:         dial,
		ForceAttemptHTTP2:   true,
		DisableCompression:  true,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 15 * time.Second,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
	}
	wsTransport := &http.Transport{
		DialContext:         dial,
		DisableCompression:  true,
		TLSHandshakeTimeout: 15 * time.Second,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}},
	}

	return &bench{
		base:        base,
		dialAddr:    dialAddr,
		headers:     hdr,
		client:      &http.Client{Transport: transport},
		wsClient:    &http.Client{Transport: wsTransport},
		timeout:     timeout,
		xferTimeout: xferTimeout,
	}, nil
}

func (b *bench) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, b.base.String()+path, body)
	if err != nil {
		return nil, err
	}
	for name, values := range b.headers {
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Encoding", "identity")
	return req, nil
}

// ping sends one GET /ping and checks that the origin answered, not an
// interstitial or error page from the tunnel provider.
func (b *bench) ping() (time.Duration, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), b.timeout)
	defer cancel()
	req, err := b.newRequest(ctx, http.MethodGet, "/ping", nil)
	if err != nil {
		return 0, "", err
	}
	start := time.Now()
	resp, err := b.client.Do(req)
	if err != nil {
		return 0, "", shortErr(err)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	elapsed := time.Since(start)
	if err != nil {
		return 0, resp.Proto, shortErr(err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, resp.Proto, fmt.Errorf("status %d: %s", resp.StatusCode, snippet(body))
	}
	if strings.TrimSpace(string(body)) != "pong" {
		return 0, resp.Proto, fmt.Errorf("unexpected body: %s", snippet(body))
	}
	return elapsed, resp.Proto, nil
}

func (b *bench) latency(n, warmup int) Latency {
	res := Latency{Requests: n}
	errs := newErrorCounter()

	// The first request includes DNS, TCP and TLS setup; record it apart.
	if d, _, err := b.ping(); err == nil {
		res.ColdMs = ms(d)
	}
	for i := 0; i < warmup; i++ {
		_, _, _ = b.ping()
	}

	var samples []float64
	for i := 0; i < n; i++ {
		d, proto, err := b.ping()
		if proto != "" {
			res.Protocol = proto
		}
		if err != nil {
			res.Errors++
			errs.add(err)
			continue
		}
		res.OK++
		samples = append(samples, ms(d))
	}
	res.Stats = computeStats(samples)
	res.ErrorLog = errs.list()
	return res
}

// concurrency sends n requests over the given number of workers, but stops
// starting new ones after maxDur, so a very slow tunnel costs bounded time.
// req/s is completed requests over elapsed time either way.
func (b *bench) concurrency(workers, n int, maxDur time.Duration) Concurrency {
	res := Concurrency{Workers: workers, Requests: n}
	errs := newErrorCounter()

	jobs := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		jobs <- struct{}{}
	}
	close(jobs)

	var mu sync.Mutex
	var samples []float64
	var wg sync.WaitGroup
	start := time.Now()
	deadline := start.Add(maxDur)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				if time.Now().After(deadline) {
					mu.Lock()
					res.TimeCapped = true
					mu.Unlock()
					return
				}
				d, _, err := b.ping()
				mu.Lock()
				res.Sent++
				if err != nil {
					res.Errors++
					errs.add(err)
				} else {
					res.OK++
					samples = append(samples, ms(d))
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start).Seconds()

	res.DurationSec = round2(elapsed)
	if elapsed > 0 {
		res.ReqPerSec = round2(float64(res.OK) / elapsed)
	}
	res.Stats = computeStats(samples)
	res.ErrorLog = errs.list()
	return res
}

func (b *bench) download(n int64) Transfer {
	t := Transfer{Bytes: n}
	ctx, cancel := context.WithTimeout(context.Background(), b.xferTimeout)
	defer cancel()
	req, err := b.newRequest(ctx, http.MethodGet, fmt.Sprintf("/bytes?n=%d", n), nil)
	if err != nil {
		t.Error = err.Error()
		return t
	}
	start := time.Now()
	resp, err := b.client.Do(req)
	if err != nil {
		t.Error = shortErr(err).Error()
		return t
	}
	defer resp.Body.Close()
	t.TTFBMs = ms(time.Since(start))
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		t.Error = fmt.Sprintf("status %d: %s", resp.StatusCode, snippet(body))
		return t
	}
	got, err := io.Copy(io.Discard, resp.Body)
	secs := time.Since(start).Seconds()
	if err != nil {
		t.Error = fmt.Sprintf("after %d bytes: %v", got, shortErr(err))
		t.Received = got
		t.Seconds = round3(secs)
		t.PartialMBps = round2(float64(got) / 1e6 / max(secs, 1e-6))
		return t
	}
	if got != n {
		t.Error = fmt.Sprintf("got %d bytes, want %d", got, n)
		return t
	}
	t.OK = true
	t.Seconds = round3(secs)
	t.MBps = round2(float64(n) / 1e6 / max(secs, 1e-6))
	return t
}

func (b *bench) upload(n int64) Transfer {
	t := Transfer{Bytes: n}
	ctx, cancel := context.WithTimeout(context.Background(), b.xferTimeout)
	defer cancel()
	var sent atomic.Int64
	body := func() io.ReadCloser {
		sent.Store(0)
		return io.NopCloser(&countingReader{r: io.LimitReader(&repeatReader{buf: payload}, n), n: &sent})
	}
	req, err := b.newRequest(ctx, http.MethodPost, "/upload", body())
	if err != nil {
		t.Error = err.Error()
		return t
	}
	req.ContentLength = n
	req.GetBody = func() (io.ReadCloser, error) { return body(), nil }
	req.Header.Set("Content-Type", "application/octet-stream")

	start := time.Now()
	resp, err := b.client.Do(req)
	if err != nil {
		secs := time.Since(start).Seconds()
		t.Error = shortErr(err).Error()
		// Bytes handed to the connection, not confirmed by the origin.
		if got := sent.Load(); got > 0 && got < n {
			t.Received = got
			t.Seconds = round3(secs)
			t.PartialMBps = round2(float64(got) / 1e6 / max(secs, 1e-6))
		}
		return t
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	secs := time.Since(start).Seconds()
	if err != nil {
		t.Error = shortErr(err).Error()
		return t
	}
	if resp.StatusCode != http.StatusOK {
		t.Error = fmt.Sprintf("status %d: %s", resp.StatusCode, snippet(raw))
		return t
	}
	var ack struct {
		Bytes int64 `json:"bytes"`
	}
	if err := json.Unmarshal(raw, &ack); err != nil {
		t.Error = "unexpected body: " + snippet(raw)
		return t
	}
	if ack.Bytes != n {
		t.Error = fmt.Sprintf("origin received %d bytes, want %d", ack.Bytes, n)
		return t
	}
	t.OK = true
	t.Seconds = round3(secs)
	t.MBps = round2(float64(n) / 1e6 / max(secs, 1e-6))
	return t
}

// maxWSReconnects bounds how often a dropped WebSocket is reopened before the
// remaining messages are counted as failures.
const maxWSReconnects = 3

func (b *bench) websocket(n, size int) WSResult {
	res := WSResult{Messages: n, MessageBytes: size, StayedUp: true}
	errs := newErrorCounter()

	wsURL := *b.base
	wsURL.Scheme = map[string]string{"http": "ws", "https": "wss"}[b.base.Scheme]
	wsURL.Path += "/ws"

	dial := func() (*websocket.Conn, error) {
		ctx, cancel := context.WithTimeout(context.Background(), b.timeout)
		defer cancel()
		h := b.headers.Clone()
		h.Set("User-Agent", userAgent)
		start := time.Now()
		c, resp, err := websocket.Dial(ctx, wsURL.String(), &websocket.DialOptions{
			HTTPClient:      b.wsClient,
			HTTPHeader:      h,
			CompressionMode: websocket.CompressionDisabled,
		})
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		if err != nil {
			return nil, shortErr(err)
		}
		if res.ConnectMs == 0 {
			res.ConnectMs = ms(time.Since(start))
		}
		c.SetReadLimit(int64(size) + 1024)
		return c, nil
	}

	conn, err := dial()
	if err != nil {
		res.StayedUp = false
		res.Failures = n
		errs.add(err)
		res.ErrorLog = errs.list()
		return res
	}

	var samples []float64
	for i := 0; i < n; i++ {
		msg := payload[(i*size)%(len(payload)-size):][:size]
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		start := time.Now()
		err := conn.Write(ctx, websocket.MessageBinary, msg)
		var got []byte
		if err == nil {
			_, got, err = conn.Read(ctx)
		}
		cancel()
		if err == nil && !bytes.Equal(got, msg) {
			err = errors.New("echo did not match")
		}
		if err == nil {
			res.Echoed++
			samples = append(samples, ms(time.Since(start)))
			continue
		}

		res.Failures++
		res.StayedUp = false
		errs.add(shortErr(err))
		conn.CloseNow()
		if res.Reconnects >= maxWSReconnects {
			res.Failures += n - i - 1
			conn = nil
			break
		}
		res.Reconnects++
		if conn, err = dial(); err != nil {
			errs.add(err)
			res.Failures += n - i - 1
			conn = nil
			break
		}
	}
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
	res.Stats = computeStats(samples)
	res.ErrorLog = errs.list()
	return res
}

// redactHost keeps the provider's domain but hides the tunnel's own label,
// e.g. "abc123.goport.uz" becomes "*.goport.uz". Hosted tunnels often reuse a
// name tied to an account, and results get published.
func redactHost(hostport string) string {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		host, port = hostport, ""
	}
	if net.ParseIP(host) == nil {
		if labels := strings.Split(host, "."); len(labels) >= 3 {
			labels[0] = "*"
			host = strings.Join(labels, ".")
		}
	}
	if port != "" {
		return net.JoinHostPort(host, port)
	}
	return host
}

func probeTCP(addr string, attempts int) Probe {
	p := Probe{Addr: addr, Attempts: attempts}
	var samples []float64
	var lastErr error
	for i := 0; i < attempts; i++ {
		start := time.Now()
		c, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			lastErr = err
			continue
		}
		samples = append(samples, ms(time.Since(start)))
		c.Close()
		time.Sleep(100 * time.Millisecond)
	}
	p.OK = len(samples)
	if s := computeStats(samples); s != nil {
		p.MinMs = s.Min
		p.MedianMs = s.Median
	}
	if lastErr != nil {
		p.Error = shortErr(lastErr).Error()
	}
	return p
}

type countingReader struct {
	r io.Reader
	n *atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

type repeatReader struct {
	buf []byte
	off int
}

func (r *repeatReader) Read(p []byte) (int, error) {
	n := copy(p, r.buf[r.off:])
	r.off = (r.off + n) % len(r.buf)
	return n, nil
}

type errorCounter struct {
	counts map[string]int
	order  []string
}

func newErrorCounter() *errorCounter { return &errorCounter{counts: map[string]int{}} }

func (e *errorCounter) add(err error) {
	msg := err.Error()
	if _, ok := e.counts[msg]; !ok {
		e.order = append(e.order, msg)
	}
	e.counts[msg]++
}

func (e *errorCounter) list() []ErrorCount {
	out := make([]ErrorCount, 0, len(e.order))
	for _, msg := range e.order {
		out = append(out, ErrorCount{Error: msg, Count: e.counts[msg]})
	}
	// Keep the file readable if every request failed differently.
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}

// shortErr drops the request URL that net/http wraps around errors, so the
// same failure on different requests counts as one kind of error.
func shortErr(err error) error {
	var uerr *url.Error
	if errors.As(err, &uerr) {
		err = uerr.Err
	}
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	return errors.New(msg)
}

func snippet(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	if s == "" {
		return "(empty)"
	}
	return s
}

func ms(d time.Duration) float64 { return round2(float64(d) / float64(time.Millisecond)) }

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
func round3(v float64) float64 { return float64(int64(v*1000+0.5)) / 1000 }
