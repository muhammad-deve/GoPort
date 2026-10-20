# Roadmap

This page shows where GoPort is heading. It isn't a commitment, and nothing on it has a date. Released changes are listed in [CHANGELOG.md](CHANGELOG.md).

To suggest something or comment on an item, [open an issue](https://github.com/muhammad-deve/GoPort/issues).

## Shipped

- **HTTPS tunnels to localhost.** `goport http <port>` gives you a public URL on a `goport.uz` subdomain. The subdomain is kept across restarts, and `--reset` gets a new random one.
- **Local request inspector.** It runs at `127.0.0.1:4040` and lets you view each request and response, replay it, or copy it as cURL.
- **WebSocket support** (0.1.3).
- **Account tokens.** Save one with `goport auth <token>`; one token per device (0.1.1).
- **Pro plan** (0.1.3): custom subdomains (`--custom`), more tunnels, traffic and devices.
- **Automatic reconnect** after a dropped connection, keeping the same URL (0.1.8).
- **Installers:**
  - release binaries for Windows, macOS and Linux (0.1.4)
  - a Windows installer (0.1.6)
  - Homebrew, Scoop and Chocolatey packages
- **Go library** `github.com/muhammad-deve/goport` for opening tunnels from Go code (0.1.1).
- **Self-hosting** the full stack with Docker Compose.
- **Security and trust** (on `main`, not yet in a release):
  - a security policy and a security model
  - govulncheck and CodeQL in CI
  - reproducible benchmarks

## Currently working on

- **Folder and media sharing (planning).** `goport share <folder_name>` returns a public link, and anyone who opens it can browse that folder's files and media.

## Planned / considering

- **Password-protected URLs.** Optionally set a password on a public URL, so only people who enter it can open the tunnel.
- **Encrypted tunnel link.** TLS between the CLI and the server on port 7000, which is plain TCP today. See [docs/security-model.md](docs/security-model.md).
- **Request inspector hardening.** A per-session secret, plus `Host` and `Origin` checks.
- **Other known weaknesses** listed in the security model:
  - storing tokens hashed on the server
  - streaming large bodies instead of buffering them in memory
- **Hosted dashboard features:**
  - custom domains with DNS verification
  - request history and replay in the dashboard
  - invoices
  - self-service account management
