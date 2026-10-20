# Changelog

Notable changes to GoPort. Versions are the tags of this repository, which also set the CLI version. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `SECURITY.md` with the vulnerability reporting process.
- `docs/security-model.md`, which explains what each connection protects and what the server can see and log.
- `CONTRIBUTING.md` and this changelog.
- CI for the root Go library (`go vet` and `go test`), govulncheck, CodeQL and Dependabot.
- A reproducible benchmark tool and method in `benchmarks/`.

## [0.1.9] - 2026-10-17

### Changed

- After a dropped connection, the CLI retries every few seconds for the first minute, then backs off. A tunnel comes back within seconds of a server restart.

### Added

- The CLI reports its version, OS, CPU architecture, install method and a one-word reconnect reason when it registers. The server stores these on the token, with the country looked up from the connecting IP address.
- The server refuses custom subdomains that contain blocked words.
- The server records how many minutes each tunnel and account stays connected.

### Fixed

- Custom subdomains created before they became Pro-only stay usable by their owner.

## [0.1.8] - 2026-10-17

### Added

- The CLI reconnects automatically when the connection drops, for example on a Wi-Fi drop, sleep or server restart, and keeps the same subdomain.

### Security

- Creating, updating and deleting tunnels and tokens through PocketBase's generic REST API is now limited to superusers. Before this, those collection rules were open.

## [0.1.7] - 2026-10-07

### Changed

- The Windows installer carries the GoPort logo.

## [0.1.6] - 2026-10-07

### Added

- A Windows installer (`goport-windows-setup.exe`) that installs without admin rights and adds `goport` to your PATH.
- The inspector shows your plan's monthly traffic allowance.

### Fixed

- Fixed inspector scrolling and the feedback after a replay.

## [0.1.5] - 2026-09-22

### Fixed

- The inspector escapes request bodies and quotes values in copied cURL commands.

## [0.1.4] - 2026-09-14

### Fixed

- Fixed the release workflow (pnpm setup).

## [0.1.3] - 2026-09-14

### Added

- Pro plan with billing through Lemon Squeezy, and usage analytics in the dashboard.
- The release workflow publishes CLI binaries for Windows, macOS and Linux.

### Changed

- `www.goport.uz` redirects to `goport.uz`.
- Free plan limits are aligned across the CLI, server and website.

## [0.1.2] - 2026-06-16

Re-tag of 0.1.1 with no code changes.

## [0.1.1] - 2026-06-16

### Added

- The Go library `github.com/muhammad-deve/goport` for opening tunnels from Go code.

### Changed

- The CLI requires an account token. Run `goport auth <token>` once.

## [0.1.0] - 2026-06-05

First tagged release.

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
[0.1.0]: https://github.com/muhammad-deve/GoPort/releases/tag/v0.1.0
