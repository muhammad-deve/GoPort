// Command tunnelbench measures latency, throughput and WebSocket behaviour of
// localhost tunnels (GoPort, ngrok, Cloudflare Tunnel, LocalTunnel, frp, zrok)
// against the same local origin server.
//
//	tunnelbench origin -addr 127.0.0.1:18080
//	tunnelbench run -url https://abc.example.com -target goport-hosted -out results/x.json
//	tunnelbench report results/2026-10-20/*.json > results/2026-10-20/SUMMARY.md
package main

import (
	"fmt"
	"os"
)

const userAgent = "tunnelbench/1"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "origin":
		err = runOrigin(os.Args[2:])
	case "run":
		err = runBench(os.Args[2:])
	case "report":
		err = runReport(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "tunnelbench:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `tunnelbench measures localhost tunnels against one local origin.

Commands:
  origin  -addr 127.0.0.1:18080
          Serve /ping, /bytes?n=N, /upload (POST) and /ws (WebSocket echo).

  run     -url BASE -target NAME -out FILE.json [flags]
          Run every phase against BASE and write the result as JSON.
          See "tunnelbench run -h" for all flags.

  report  FILE.json...
          Print a markdown summary of the given result files.
`)
}
