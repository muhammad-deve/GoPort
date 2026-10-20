# Tunnel benchmarks

Reproducible measurements of GoPort and other localhost tunnels (ngrok, Cloudflare Tunnel, LocalTunnel, frp, zrok) against the same local origin server.

Read this first: **every number here comes from one machine on one network in one city.** A tunnel's latency mostly depends on how far your machine, the relay and your visitor are from each other, and much less on the tunnel software. A relay 70 ms away will beat a relay 120 ms away almost no matter how either is written. Use these results to see how the tools behaved from this vantage point, and run the scripts from your own location before drawing conclusions.

- [What is measured](#what-is-measured)
- [Targets](#targets)
- [Run it yourself](#run-it-yourself)
- [Results: 2026-10-20](#results-2026-10-20)
- [Limitations](#limitations)

## What is measured

`tunnelbench` (in `cmd/tunnelbench`) runs a small origin server on `127.0.0.1` and measures it through each tunnel's public URL. Every run does the same five phases, in order:

| Phase | What it does | Reported |
|---|---|---|
| TCP connect | 5 TCP connects to the public URL's host (the provider's edge), plus any extra `-probe` addresses such as GoPort's `goport.uz:7000` | median connect time |
| Latency | 200 sequential `GET /ping` over a kept-alive connection, after 5 uncounted warm-up requests. The first request (DNS + TCP + TLS) is recorded separately as `cold_ms` | min, median, mean, p95, p99, max, errors |
| Concurrency | Up to 1,000 `GET /ping` from 10 workers. The phase stops starting new requests after 90 s, so a slow tunnel costs bounded time | completed requests per second, p50/p95/p99, errors |
| Download | 3 × `GET /bytes?n=10000000` (10 MB of random, incompressible data) | MB/s per transfer (10⁶ bytes/s), including time to first byte |
| Upload | 1 × `POST /upload` with 10 MB | MB/s |
| WebSocket | Opens `/ws` (wss for https URLs) and does 100 echo round trips of 1 KiB | RTT median/p95, failures, whether the connection stayed up |

Details that keep the comparison fair:

- Every response is checked. `/ping` must return `pong`, transfers must deliver exactly the requested byte count. An interstitial or error page from a provider counts as an error, not a fast response.
- Compression is disabled (`Accept-Encoding: identity`) and the payload is random, so no proxy can shrink the transfer.
- HTTP/2 is used when the provider offers it, like a browser would. The protocol seen is recorded.
- Transfers time out after 120 s. A timed-out transfer is a failure, and the rate it reached before the timeout is recorded as `partial_mb_per_sec`.
- Failures are recorded, never dropped. If a tunnel doesn't come up at all, the script writes a run with every phase failed (`-setup-error`).
- `run-all.sh` interleaves targets: each round runs every target once, and the order rotates between rounds. A bad few minutes on the network then hits every tool, instead of whichever one ran then.
- The User-Agent is `tunnelbench/1`. ngrok's free browser warning and LocalTunnel's reminder page are skipped with the headers each provider documents for this (`ngrok-skip-browser-warning`, `bypass-tunnel-reminder`).
- Hosted tunnel hostnames are stored with the first label hidden (`*.goport.uz`), since some providers reuse one name per account.

## Targets

| Script | Target name | What it measures | Needs |
|---|---|---|---|
| `baseline.sh` | `baseline-localhost` | The origin directly over loopback. Everything else is measured against this | nothing |
| `goport-hosted.sh` | `goport-hosted` | GoPort's hosted service, `*.goport.uz` | a GoPort account token (`GOPORT_TOKEN` or `goport auth`) |
| `goport-selfhosted.sh` | `goport-selfhosted` | GoPort CLI against your own GoPort server | your server, `GOPORT_SERVER_ADDR`, `GOPORT_DOMAIN`, `GOPORT_API_URL`, a token from it |
| `goport-selfhosted.sh` with `SELFHOSTED_CONNECT_URL` | `goport-selfhosted-loopback` | GoPort server and CLI on the same machine, plain HTTP, no nginx or TLS. Software overhead only | a local GoPort backend (see below) |
| `ngrok.sh` | `ngrok` | ngrok's hosted service | ngrok agent with an authtoken |
| `cloudflared.sh` | `cloudflared-quick` | Cloudflare Tunnel in quick-tunnel mode (`trycloudflare.com`, no account) | `cloudflared` |
| `localtunnel.sh` | `localtunnel` | LocalTunnel's public server (`*.loca.lt`) | Node.js (`npx`) |
| `frp.sh` | `frp-loopback` | frps and frpc on the same machine. Software overhead only | `frps`, `frpc` |
| `frp.sh` with `FRP_MODE=remote` | `frp-remote` | frpc against your own frps | your frps |
| `zrok.sh` | `zrok` | zrok public share | `zrok enable` done beforehand |

Loopback targets (`*-loopback`) measure what the tunnel software adds when the network is taken out of the picture. Don't compare them with the hosted targets, which add a real network path.

## Run it yourself

You need Go 1.23 or newer, bash (Git Bash works on Windows), `curl`, and whichever tunnel clients you want to test.

```bash
cd benchmarks

# One target, one round
bash scripts/baseline.sh
bash scripts/cloudflared.sh

# Several targets, 3 interleaved rounds, then a summary
ROUNDS=3 bash scripts/run-all.sh baseline goport-hosted ngrok cloudflared localtunnel frp

# Summary of any set of results
go run ./cmd/tunnelbench report results/2026-10-20/*.json > results/2026-10-20/SUMMARY.md
```

Useful environment variables (see the header of each script for the rest):

| Variable | Default | Meaning |
|---|---|---|
| `ROUNDS` | `3` | rounds for `run-all.sh` |
| `OUT_DIR` | `results/<UTC date>` | where result JSON goes |
| `WORK_DIR` | `$TMPDIR/tunnelbench` | logs, generated configs, the built tool |
| `ORIGIN_PORT` | `18080` | local origin port |
| `BENCH_ARGS` | empty | extra flags for `tunnelbench run`, e.g. `-latency-requests 50` |
| `GOPORT_BIN`, `NGROK_BIN`, `CLOUDFLARED_BIN`, `FRPC_BIN`, `FRPS_BIN`, `ZROK_BIN` | on `PATH` | client binaries |

`tunnelbench` can also be used directly:

```bash
go run ./cmd/tunnelbench origin -addr 127.0.0.1:18080
go run ./cmd/tunnelbench run -url https://abc.example.com -target my-tunnel -out results/x.json
go run ./cmd/tunnelbench run -h    # every flag and its default
```

### A local GoPort server for the loopback target

This is how the `goport-selfhosted-loopback` numbers below were produced, without changing the backend code. Run it from a directory that has no `.env` in it or its three parent directories, because the backend loads the first one it finds.

```bash
cd backend/app && go build -o /tmp/goport-backend ./cmd/main.go
cd /tmp/some/empty/dir
/tmp/goport-backend superuser upsert admin@example.test '<password>' --dir=/tmp/pb_data
TCP_PORT=7000 GOPORT_DOMAIN=bench.test /tmp/goport-backend serve --http=127.0.0.1:8090 --dir=/tmp/pb_data
```

Then create a user through the PocketBase admin API (`POST /api/collections/users/records` with a superuser token). The backend adds a `default` device token for every new user, which you can read from `/api/collections/tokens/records`. Point the CLI at it and run the script:

```bash
export GOPORT_SERVER_ADDR=127.0.0.1:7000 GOPORT_DOMAIN=bench.test GOPORT_API_URL=http://127.0.0.1:8090
export GOPORT_TOKEN=<that token> SELFHOSTED_CONNECT_URL=http://127.0.0.1:8090
bash scripts/goport-selfhosted.sh
```

The tunnel server listens on all interfaces (`:7000`), so on Windows the firewall may ask whether to allow it. Loopback traffic works either way.

## Results: 2026-10-20

Full results, including every run and every error: [`results/2026-10-20/SUMMARY.md`](results/2026-10-20/SUMMARY.md). Raw JSON for each run is next to it.

### Setup

| | |
|---|---|
| Date | 2026-10-20, 11:31–12:07 UTC, 3 interleaved rounds |
| Machine | Intel Core i7-11800H (8 cores / 16 threads), 15.7 GB RAM |
| OS | Windows 11 Pro 10.0.26200 |
| Tool | `tunnelbench` built with Go 1.25.5 |
| Network | Residential connection in Tashkent, Uzbekistan. ICMP ping just before the run: `goport.uz` 117–130 ms (avg 122), `api.trycloudflare.com` 67–74 ms (avg 69), `loca.lt` 195–201 ms (avg 197). TCP connect to `goport.uz:7000` 126–134 ms |
| Visitor | The same machine, through each tunnel's public URL |

| Target | Client version | Relay / region |
|---|---|---|
| `baseline-localhost` | none | loopback |
| `goport-selfhosted-loopback` | GoPort CLI 0.1.9 (release binary, SHA-256 checked against `checksums.txt`) and the backend built from `backend/` at commit `e1d9b7d` | same machine, plain HTTP, no nginx or TLS |
| `frp-loopback` | frp 0.68.1 | same machine |
| `ngrok` | ngrok agent 3.39.9 (free account, default settings) | automatic region; `ngrok diagnose` reported `in` (~200 ms) before each of the three runs, and `eu` (~102 ms) in a separate check earlier that day. Edge TCP connect ~100 ms |
| `cloudflared-quick` | cloudflared 2026.10.2 (official release, SHA-256 checked) | quick tunnel; connector edges `waw02`, `waw03`, `waw06` (Warsaw). Edge TCP connect ~80 ms |
| `localtunnel` | localtunnel 2.0.2 (npm), public server | `*.loca.lt`, edge TCP connect ~205 ms |

### Median of three rounds

Latency and WebSocket figures are in milliseconds, throughput in MB/s (10⁶ bytes per second). Each cell is the median of the three runs' values. Errors are totals over all three runs.

| Target | Latency median | p95 | p99 | Concurrent req/s (10 workers) | Concurrent errors | Download MB/s | Upload MB/s | WebSocket RTT median | WebSocket failures |
|---|---|---|---|---|---|---|---|---|---|
| baseline-localhost | <0.5 | 0.52 | 0.53 | 62,624 | 0 / 3000 | 3,685 | 2,667 | <0.5 | 0 / 300 |
| goport-selfhosted-loopback | 0.52 | 1.07 | 2.55 | 3,336 | 0 / 3000 | 241 | 115 | <0.5 | 0 / 300 |
| frp-loopback | <0.5 | 0.53 | 0.63 | 18,888 | 0 / 3000 | 686 | 326 | <0.5 | 0 / 300 |
| ngrok | 197.5 | 211.3 | 489.3 | 43.5 | 0 / 3000 | 2.14 | 0.88 | 202.4 | 0 / 300 |
| cloudflared-quick | 176.3 | 200.6 | 220.5 | 56.1 | 0 / 3000 | 1.67 | 0.93 | 169.2 | 0 / 300 |
| localtunnel | 803.5 | 1917.0 | 2303.3 | 2.4 | 32 / 749 (HTTP 502) | 2.85 | 2.55 | 432.2 | 0 / 300 |

Every transfer completed (no timeouts), every WebSocket connection stayed up for all 100 echoes, and the only errors were LocalTunnel's 502 responses under concurrency. LocalTunnel's concurrency phase hit the 90-second cap after about 250 requests per run.

What this run shows, and what it doesn't:

- **Hosted GoPort was not measured** (see [Limitations](#limitations)), so this run says nothing about goport.uz's latency or throughput compared with ngrok or Cloudflare. For context only: the round trip from this machine to the goport.uz server was ~122 ms, versus ~69 ms to Cloudflare's nearest edge and ~100 ms to ngrok's.
- **Software overhead on loopback.** With the network taken out, GoPort 0.1.9 added about half a millisecond at the median and reached ~3,300 requests/s with 10 workers, against ~18,900 for frp on the same machine. Its 10 MB download ran at ~240 MB/s against frp's ~690 MB/s. Neither matters much once a real network path is involved: the hosted targets' medians were all under 3 MB/s and under 60 requests/s.
- **Hosted latency tracks distance.** Cloudflare's quick tunnel had the nearest edge and the lowest latency. For ngrok, `ngrok diagnose` named its `in` region as the closest at the time. LocalTunnel's server was the farthest away and the slowest per request and under load, although its single-stream downloads were the fastest of the three hosted services.
- **Throughput varied between runs.** ngrok's downloads were ~2.1–2.2 MB/s in rounds 1 and 2 and 0.6–0.9 MB/s in round 3. Cloudflare's three downloads in each run rose from ~1.0 to ~2.0 MB/s as the connection warmed up. See the per-run table in `SUMMARY.md`.
- **Timer resolution.** On Windows, Go's clock advances in steps of about 0.5 ms, so loopback figures below that read as 0.00 (shown here as `<0.5`). Remote results are unaffected in practice.

## Limitations

- **One vantage point.** One machine, one residential connection, one city (Tashkent). Run from Europe or the US and the order between the hosted services may change completely.
- **Geography dominates.** GoPort's hosted server is in France, roughly 120 ms from where this ran; Cloudflare's nearest edge was about 70 ms away; LocalTunnel's server about 200 ms. A tunneled request crosses the relay link twice (visitor → relay → your machine and back), so a relay's distance roughly doubles in the result.
- **Visitor and tunnel client were the same machine.** In real use the visitor (a webhook provider, a phone, a teammate) is somewhere else, which changes the visitor-to-relay leg.
- **Small sample.** Three rounds per target within about 90 minutes. Time-of-day effects on a residential line and on the providers' shared free infrastructure are not averaged out.
- **Free tiers.** ngrok, Cloudflare quick tunnels and LocalTunnel were used on their free tiers or without accounts. Free tiers can be rate-limited or throttled, and paid plans may behave differently. Cloudflare documents quick tunnels as a testing tool without an uptime guarantee.
- **Loopback runs measure software overhead only.** `frp-loopback` and `goport-selfhosted-loopback` take the network out entirely, and the GoPort loopback run also skips nginx and TLS, which a real deployment has in front of it.
- **What GoPort does with bodies.** As of v0.1.9 the CLI reads each HTTP request body and each response body fully into memory before passing it on, and the server does the same with request bodies (responses it streams). For a large download that delays the first byte until the CLI has the whole response from your app. WebSocket and other upgraded connections are streamed both ways.
- **Not measured on 2026-10-20:**
  - `goport-hosted`: the GoPort token saved on the benchmark machine was rejected by goport.uz (HTTP 401), and no other account was used. Run `GOPORT_TOKEN=<token> bash scripts/goport-hosted.sh` to fill this in.
  - `goport-selfhosted` (remote) and `frp-remote`: no second server was available.
  - `zrok`: not installed and needs an account (`zrok invite`, `zrok enable`).
- **Versions move.** Every result records the client version. Re-run before quoting numbers for a newer release.
