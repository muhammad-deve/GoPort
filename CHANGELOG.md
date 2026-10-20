# Changelog

Notable changes to GoPort, following [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/).

- **Versions:** each version is a git tag of this repository, and the tag also sets the CLI version.
- **Scope:** each entry covers the CLI, the tunnel server and the hosted service as they were at that tag.
- **Release pages:** GitHub Release pages exist from 0.1.4. Versions 0.1.0 to 0.1.3 are git tags only.

## [Unreleased]

### Added

- `SECURITY.md` and [`docs/security-model.md`](docs/security-model.md): how to report a vulnerability, what each connection protects, and what the server can see and log.
- `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1), `SUPPORT.md`, `ROADMAP.md` and this changelog.
- CI checks:
  - `go vet` and `go test` for the root library and the benchmark tool
  - govulncheck
  - CodeQL for Go, JavaScript/TypeScript and Actions
  - Dependabot
- `benchmarks/`: a tool and scripts that measure tunnel latency, throughput and WebSocket behaviour, plus the 2026-10-20 results.

### Changed

- The deploy workflow for this repository no longer rebuilds the backend or the website. Each one deploys from its own repository.

## [0.1.9] - 2026-10-17

### Added

- The CLI reports its version, OS, CPU architecture, install method and a reconnect reason when it registers. The server stores these on the token, with the country looked up from the connecting IP address.
- The server refuses custom subdomains that contain blocked words.
- The server records how many minutes each tunnel and account stays connected.

### Changed

- After a dropped connection, the CLI retries every few seconds for the first minute, then backs off.
- The README now follows the install, log in and run steps.

### Fixed

- Custom subdomains created before they became Pro-only stay usable by their owner.

## [0.1.8] - 2026-10-17

### Added

- The CLI reconnects automatically when the connection drops and keeps the same subdomain.

### Security

- PocketBase's REST API exposed the tunnel and token collections:
  - Anyone could create records in them, even when signed out.
  - That allowed creating a token for another user's account.
  - Owners could edit tunnel records to get around plan checks.

  Creating, updating and deleting those records is now limited to superusers.

## [0.1.7] - 2026-10-07

### Changed

- The Windows installer uses GoPort branding.

## [0.1.6] - 2026-10-07

### Added

- A Windows installer, `goport-windows-setup.exe`. It installs per user without admin rights and adds `goport` to your PATH.
- The inspector shows your plan's monthly traffic allowance and usage.

### Fixed

- Scrolling and replay feedback in the inspector.

## [0.1.5] - 2026-09-22

### Added

- Hosted billing shows trial status, a link to the billing portal and monthly usage.

### Changed

- The inspector's design now matches goport.uz.

### Security

- The inspector inserted request and response bodies into the page as HTML without escaping. Anyone who knew a tunnel URL could therefore run script in the owner's inspector. Bodies are now escaped.
- "Copy as cURL" wrapped header values in double quotes, so a crafted header could run a command when pasted into a shell. Headers and bodies are now single-quoted.

## [0.1.4] - 2026-09-14

### Fixed

- The release workflow (pnpm setup). This is the first version published as a GitHub Release with downloadable binaries.

## [0.1.3] - 2026-09-14

### Added

- Pro plan with subscriptions billed through Lemon Squeezy.
- Tunnel controls and usage history in the dashboard.
- A release workflow that builds CLI binaries for Windows, macOS and Linux.

### Changed

- The Free plan allows one active tunnel.
- Custom subdomains (`--custom`) require Pro.
- `www.goport.uz` redirects to `goport.uz`.

### Fixed

- WebSocket connections through the tunnel. The CLI removed the `Upgrade` headers, so WebSocket handshakes reached the app as plain `GET` requests.
- The dashboard's online status for tunnels. WebSocket traffic now also counts toward usage.

### Security

- Card brand and last four digits are no longer stored, and previously stored values were deleted.

## [0.1.2] - 2026-06-16

Same commit as 0.1.1, so there are no changes.

## [0.1.1] - 2026-06-16

### Added

- `goport auth <token>` saves an account token. Tokens are created in the dashboard.
- Dashboard sign-in with Google, and email verification codes.
- The Go library `github.com/muhammad-deve/goport` for opening tunnels from Go code.
- README and MIT license.

### Changed

- Opening a tunnel requires an account token.

## [0.1.0] - 2026-06-05

### Added

- `goport http <port>` gives a local app a public HTTPS URL on a goport.uz subdomain, over one outbound TCP connection multiplexed with yamux.
- `--custom` / `-n` to request a subdomain, and `--reset` for a new random one.
- A local request inspector with replay.
- Docker Compose and nginx configuration for running the server.

[Unreleased]: https://github.com/muhammad-deve/GoPort/compare/v0.1.9...HEAD
[0.1.9]: https://github.com/muhammad-deve/GoPort/compare/v0.1.8...v0.1.9
[0.1.8]: https://github.com/muhammad-deve/GoPort/compare/v0.1.7...v0.1.8
[0.1.7]: https://github.com/muhammad-deve/GoPort/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/muhammad-deve/GoPort/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/muhammad-deve/GoPort/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/muhammad-deve/GoPort/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/muhammad-deve/GoPort/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/muhammad-deve/GoPort/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/muhammad-deve/GoPort/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/muhammad-deve/GoPort/tree/v0.1.0
