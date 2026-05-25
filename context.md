What GoPort is
Ngrok-like tunnel platform. User runs goport http 8080 → their localhost becomes public via abc123.goport.uz.
Architecture decided: Option B (TCP + yamux)

Not HTTP proxy (Option A)
Real TCP multiplexing like ngrok
One persistent TCP connection CLI→Server
yamux runs multiple streams over that one connection
3 repos

server → CLI binary (what you're building now)
backend → API, auth, user management, tunnel registry
landing → goport.uz landing page How it works (your mental model, now correct)
1. CLI dials backend :7000 (CLI is TCP client, backend is TCP server)
2. JSON handshake over that connection
   CLI sends → { token, port }
   Backend replies → { subdomain, url }
3. Backend wraps conn with yamux.Server()
   CLI wraps conn with yamux.Client()
4. Backend stores in memory: tunnels["abc123"] = yamuxSession
5. Browser hits abc123.goport.uz
   Backend reads Host header → looks up session
   session.Open() → new stream
   CLI's session.Accept() unblocks → goroutine
   CLI dials localhost:8080
   io.Copy both directions
Key concepts you now understand

Server never dials CLI — CLI always initiates
DB only touched once (registration) — all routing is in-memory map
yamux = framing protocol, adds 12-byte header with stream ID so multiple requests don't mix over one TCP pipe
TCP connection lives only while goport http 8080 is running
On CLI disconnect → delete from tunnels map or dead sessions pile up Current CLI structure
server/
├── main.go          ← calls cmd.Execute()
├── cmd/
│   ├── root.go      ← rootCmd + Execute()
│   ├── http.go      ← httpCmd, flags: -n (name), -r (region)
│   └── tcp.go       ← tcpCmd
└── tunnel/
    └── tunnel.go    ← Config struct + Start() + net.Dial(:7000)
Current state of tunnel.go
go

func Start(cfg Config) {
    conn, err := net.Dial("tcp", "localhost:7000")
    // connects, prints success or error
}
What's working right now

goport http 8080 runs ✓
CLI tries to connect to :7000 ✓
Fails with "connection refused" — expected, backend not running yet ✓
Module name issue

Currently named github.com/muhammad-deve/server → installs as server.exe
Need to rename to github.com/muhammad-deve/goport → installs as goport.exe
Does not affect end users, internal only Next steps in order
Step 2 → Backend TCP listener on :7000 (backend repo)
Step 3 → JSON handshake both sides
Step 4 → yamux wraps connection
Step 5 → Public listener reads Host header, opens stream
Step 6 → Full end-to-end test