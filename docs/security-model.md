# GoPort security model

This document covers how GoPort moves your traffic, what each connection protects, what the server can see and keep, and what to watch for when you expose a local app. It describes the code as of CLI 0.1.9 and the `backend` `main` branch (October 2026). Where the hosted service's settings can't be read from the code, we say so.

For reporting a vulnerability, see [SECURITY.md](../SECURITY.md).

## Summary

- Visitors reach your tunnel over **HTTPS**. TLS ends at the GoPort server's reverse proxy.
- The link between the **CLI and the tunnel server (TCP port 7000) is not encrypted**. Your account token and every tunneled request and response cross it in plain text.
- GoPort is **not end-to-end encrypted**. The server sees tunneled traffic in clear text, as any TLS-terminating tunnel does.
- The tunnel server doesn't save request or response bodies or headers. With PocketBase's defaults, its request log keeps each tunneled request's path, query string, IP addresses, and user agent for 5 days.
- The **request inspector** runs only on your machine and keeps the last 500 requests in memory. It has **no login**.

## Components

| Component | Code | Role |
|---|---|---|
| CLI (`goport`) | [`server/`](https://github.com/muhammad-deve/server) (Go) | Connects out to the tunnel server, forwards requests to `127.0.0.1:<port>`, and serves the local inspector |
| Tunnel server and API | [`backend/`](https://github.com/muhammad-deve/backend) (Go, PocketBase v0.27) | Accepts CLI connections on TCP 7000, routes `*.goport.uz` requests by `Host` header, and runs the accounts, tokens and billing API |
| Website and dashboard | [`landing/`](https://github.com/muhammad-deve/landing) (Next.js) | goport.uz, sign-up and log-in, token management |
| Reverse proxy | [`nginx/goport.conf`](../nginx/goport.conf) (template) | Terminates TLS for `goport.uz`, `back.goport.uz` and `*.goport.uz`, and redirects HTTP to HTTPS |

## Connections

| # | From → to | Transport | Encrypted | What it carries |
|---|---|---|---|---|
| 1 | Visitor → reverse proxy (443) | HTTPS, TLS 1.2 or 1.3, wildcard certificate | **Yes** | The visitor's request and your app's response |
| 2 | Reverse proxy → tunnel server (HTTP port) | HTTP on the same host | No (stays on the server) | The same request after TLS is removed |
| 3 | CLI → tunnel server (TCP 7000) | Plain TCP, multiplexed with [yamux](https://github.com/hashicorp/yamux) | **No** | Registration, including your **token**, then every tunneled request and response, including WebSocket data |
| 4 | CLI → API (`https://back.goport.uz`) | HTTPS | **Yes** | Token check on `goport auth`, and a plan and usage check every 60 seconds while a tunnel runs |
| 5 | CLI → your app | HTTP to `127.0.0.1:<port>` | No (loopback only) | The forwarded request |
| 6 | Your browser → inspector | HTTP to `127.0.0.1:4040` | No (loopback only) | Captured requests and responses |

### The CLI link (connection 3)

The CLI dials the server and keeps one TCP connection open. The server never dials the CLI, which is why GoPort works behind NAT.

1. The CLI sends one JSON line. It contains the requested port and subdomain, the **account token**, a `resume` subdomain after a reconnect, and since 0.1.9 the CLI version, OS, CPU architecture, install method, and a one-word reconnect reason. It doesn't send hostnames, local paths or local IP addresses.
2. The server answers with one JSON line: the assigned subdomain and URL, or an error.
3. Both sides start a yamux session on the same connection. For each public request the server opens a stream and writes the HTTP request to it. The CLI replies on the same stream.

yamux only frames and multiplexes. It doesn't encrypt or authenticate. **Anyone who can watch or alter the network path between your machine and the server** can therefore do the following, including a shared Wi-Fi network, your ISP or a hostile network operator:

- read your token and use it to open tunnels on your account
- read every tunneled request and response
- change traffic in transit

Until this link supports TLS, treat any network you run the CLI on as able to see your tunneled traffic.

## Authentication

- **Dashboard accounts** sign in with email and password (PocketBase stores password hashes) or with Google OAuth. Sign-up and password resets confirm the email address with a one-time code, and sending codes is rate-limited.
- **CLI tokens** are created in the dashboard. Each is `gp_` followed by 32 hex characters from a random UUIDv4 (122 random bits). Each token counts as one device, and the plan limits how many you can have: 1 on Free, 5 on Pro. Delete a token in the dashboard to revoke it.
- **The server stores token values as they are, not hashed.** The dashboard shows them in full. Anyone with read access to the server's database can use them.
- **A tunnel always needs a valid token.** The server refuses a registration with a missing or unknown token.

## Token storage on your machine

`goport auth <token>` checks the token with the API over HTTPS and then saves it as plain text:

| OS | Path |
|---|---|
| Linux | `$XDG_CONFIG_HOME/goport/token`, or `~/.config/goport/token` |
| macOS | `~/Library/Application Support/goport/token` |
| Windows | `%AppData%\goport\token` |

- **Permissions:** on Linux and macOS the file is created with mode `0600` in a `0700` folder. On Windows it inherits the permissions of your user profile. The token isn't kept in the OS keychain.
- **Environment variable:** `GOPORT_TOKEN` overrides the saved file, which helps in CI. Environment variables can leak into logs and child processes, so use your CI's secret store.
- **Removing it:** `goport auth --logout` deletes the file.

## What the hosted server can see

GoPort terminates TLS on the server, so it **can read tunneled traffic in full** while relaying it. Even with an encrypted CLI link this would still be true, as with any service of this kind. That covers:

- the method, URL, query string, and all headers, including `Cookie` and `Authorization`
- request and response bodies
- WebSocket messages
- visitor IP addresses and your CLI's IP address

## What the hosted server keeps

**Stored in the database:**

- **Account data:** name, email, plan and subscription status.
- **Tokens:** value, name, creation and last-use times. Since 0.1.9 also the device details the CLI reports, the country looked up from the CLI's IP address, reconnect counts and the last disconnect reason.
- **Tunnels:** subdomain, local port number, protocol, creation time, connected minutes, and whether the tunnel is online.
- **Usage:** per-tunnel request counts, bytes transferred, and last activity, plus time buckets for the dashboard charts.

**Not stored in the database:** request or response bodies, headers, and WebSocket messages.

**Logs:**

- **PocketBase request log.** PocketBase logs every HTTP request it handles, and tunneled requests pass through it. Each entry records the method, path with query string, status, referrer, user agent, the client IP addresses, and timing. PocketBase's default retention is **5 days**, with IP logging on. Operators can change or disable this under *Settings → Logs* in the PocketBase admin UI. The code doesn't say what the hosted service is set to.
- **Process log.** The server's process log records when a CLI connects and disconnects, with its IP and port, and each registration with its subdomain and local port.
- **Reverse proxy.** The reverse proxy in front may keep its own access log. nginx does by default.

**Keep secrets out of URLs.** A token in a query string, such as `?api_key=...`, ends up in these logs.

## Request inspector

The CLI serves an inspector at `http://127.0.0.1:4040`. If 4040 is taken it uses the next free port, up to 4139.

**Storage:**

- It binds to the loopback address, so other machines on your network can't reach it.
- It keeps the **last 500** request/response pairs **in memory only**. Nothing is written to disk, and everything is gone when the CLI exits or you clear the list.
- The stored copy of each body is cut off at **256 KiB**. Forwarded traffic is never cut. The inspector keeps the first value of each header.

**Replay** sends the stored request straight to `127.0.0.1:<port>`, not through the public URL. It uses the stored, possibly truncated body and sets `Host` to `127.0.0.1:<port>`.

**No login.** Anything that can send requests to `127.0.0.1:4040` on your machine can read every captured request and response. That includes other users on a shared machine and other local programs. Stop the CLI when you're done, and don't run it on machines you share with people you don't trust.

## Risks of exposing localhost

A tunnel gives the whole internet a path to a process that was written for your machine alone.

- **Anyone with the URL can reach your app.** Random subdomains are 6 characters, about 2 billion combinations. Treat them as an address, not a password. GoPort has no built-in access control, so put authentication in your app.
- **Your app sees every visitor as local.** Requests arrive from `127.0.0.1`, the CLI. Code that trusts localhost will treat internet visitors as local: admin pages allowed only from localhost, debug consoles such as the Rails web console, or framework debuggers that run code. Turn those off before you start a tunnel.
- **Development servers expose more than production.** They often serve source maps, stack traces, environment variables, unauthenticated admin panels, and files outside the app.
- **All paths are forwarded.** Not only the route you share.
- **Bodies are buffered in memory.** The server reads each request body in full before forwarding it, and the CLI reads full request and response bodies. Large uploads or downloads use memory on your machine in proportion to their size. The hosted proxy template caps request bodies at 50 MB.

## Recommended precautions

1. **Don't tunnel production data or real credentials** until the CLI link is encrypted. Use test keys, test accounts, and webhook test modes.
2. **Stop the tunnel when you're done.** Every minute it runs, the app is public.
3. **Use a separate token per device**, and delete tokens you no longer use. If you ran the CLI on a network you don't trust, rotate your token.
4. **Check webhook signatures** (Stripe, GitHub, Telegram and others sign their deliveries) instead of trusting any request that reaches your handler.
5. **Turn off debug consoles and localhost-only admin features** before you start a tunnel.
6. **Keep secrets out of URLs** so they don't land in request logs.
7. **Update the CLI** to the latest release.

### When you self-host

- **Port 7000:** allow it only from networks you trust, or reach it over a VPN such as WireGuard, until the CLI link supports TLS.
- **PocketBase admin UI:** restrict `/_/` (allow-list your IP in the reverse proxy) and use a strong superuser password.
- **Log retention:** decide how long PocketBase should keep request logs (*Settings → Logs*). Set it to 0 to turn the request log off.
- **Plan limits:** the server applies the Free and Pro limits in `backend/app/internal/model/billing.go` to every account, including on your own server. Change them before you build if you want different limits.

## Known weaknesses and planned work

These are open. They're listed here so you can judge the risk yourself.

| Weakness | Effect | Direction |
|---|---|---|
| CLI ↔ server link is plain TCP | Token theft and traffic interception on the network path | TLS on the tunnel port, in a future CLI and server release |
| Tokens stored unhashed on the server | A database leak exposes usable tokens | Store a hash, show the token once at creation |
| Inspector has no authentication | Local processes can read captured traffic | Bind with a per-session secret and check `Host` and `Origin` |
| Whole bodies buffered in memory | Memory use grows with transfer size | Stream bodies instead of buffering |
