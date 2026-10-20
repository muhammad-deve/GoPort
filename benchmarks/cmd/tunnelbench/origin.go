package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/coder/websocket"
)

// maxBytes caps /bytes so a typo can't make the origin stream forever.
const maxBytes = 1 << 30

// payload is a block of pseudo-random bytes repeated by /bytes and used as the
// upload body. Random data keeps gzip in proxies from shrinking the transfer
// and inflating the measured throughput. The 1 MiB period is far larger than
// gzip's 32 KiB window, so repeating it doesn't make it compressible either.
var payload = func() []byte {
	b := make([]byte, 1<<20)
	rand.New(rand.NewSource(1)).Read(b)
	return b
}()

func runOrigin(args []string) error {
	fs := flag.NewFlagSet("origin", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:18080", "listen address")
	_ = fs.Parse(args)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           originHandler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	log.Printf("origin listening on http://%s", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func originHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, "pong\n")
	})

	mux.HandleFunc("/bytes", func(w http.ResponseWriter, r *http.Request) {
		n, err := strconv.ParseInt(r.URL.Query().Get("n"), 10, 64)
		if err != nil || n < 0 || n > maxBytes {
			http.Error(w, "n must be between 0 and 1073741824", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Length", strconv.FormatInt(n, 10))
		for n > 0 {
			chunk := payload
			if int64(len(chunk)) > n {
				chunk = chunk[:n]
			}
			written, err := w.Write(chunk)
			if err != nil {
				return
			}
			n -= int64(written)
		}
	})

	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		n, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{"bytes": n})
	})

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			// Tunnels rewrite or drop Origin in different ways; this test
			// server holds nothing worth protecting.
			InsecureSkipVerify: true,
			CompressionMode:    websocket.CompressionDisabled,
		})
		if err != nil {
			return
		}
		defer c.CloseNow()
		c.SetReadLimit(1 << 20)
		ctx := r.Context()
		for {
			typ, msg, err := c.Read(ctx)
			if err != nil {
				return
			}
			if err := c.Write(ctx, typ, msg); err != nil {
				return
			}
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintln(w, "tunnelbench origin: /ping /bytes?n=N /upload /ws")
	})

	return mux
}
