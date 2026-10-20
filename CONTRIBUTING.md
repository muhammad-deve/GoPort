# Contributing to GoPort

Thanks for helping. Bug reports, fixes and documentation improvements are all welcome.

## Where the code lives

This repository ties GoPort's parts together. Most code lives in separate repositories added here as git submodules:

| Folder | Repository | What it is |
|---|---|---|
| `server/` | [muhammad-deve/server](https://github.com/muhammad-deve/server) | The `goport` CLI, in Go. The folder name is historical. |
| `backend/` | [muhammad-deve/backend](https://github.com/muhammad-deve/backend) | The tunnel server and API, in Go on PocketBase |
| `landing/` | [muhammad-deve/landing](https://github.com/muhammad-deve/landing) | goport.uz and the account dashboard, in Next.js |
| `Installers/` | this repo plus the Homebrew and Scoop repos | Windows installer and package-manager manifests |
| `benchmarks/` | this repo | Tunnel benchmark tool and results |
| repository root | this repo | The Go library `github.com/muhammad-deve/goport` |

**Open pull requests against the repository that owns the code.** A change to the CLI goes to `muhammad-deve/server`, not here.

## Reporting bugs

[Open an issue](https://github.com/muhammad-deve/GoPort/issues) with:

- the command you ran
- your OS and how you installed GoPort
- the output of `goport --version`
- what you expected to happen and what happened instead, including the full error

**Security problems:** don't open an issue. Follow [SECURITY.md](SECURITY.md). For account or billing help, see [SUPPORT.md](SUPPORT.md).

## Getting the code

```bash
git clone --recursive https://github.com/muhammad-deve/GoPort.git
cd GoPort
```

If you cloned without `--recursive`, run `git submodule update --init --recursive`.

## Building and testing

**Go library (repository root):**

```bash
go vet ./...
go test ./...
```

**CLI:**

```bash
cd server
go build -o goport .
go test ./...
```

The inspector UI is a static Next.js export embedded in the binary. `make -C server dashboard` rebuilds it, which needs Node.js and pnpm.

**Tunnel server:**

```bash
cd backend/app
cp .env.example .env
go run ./cmd serve
```

**Website:**

```bash
cd landing
pnpm install
pnpm dev
```

To run a full stack on your own domain, see [goport.uz/self-hosting](https://goport.uz/self-hosting).

## Pull requests

- **Keep one change per pull request.** Explain what it changes and why.
- **Add or update tests** for behaviour changes in Go code.
- **Run `go vet` and `go test`**, plus `pnpm lint` for the website, before you push.
- **Commit messages:** follow the existing style, [Conventional Commits](https://www.conventionalcommits.org/), for example `fix(tunnel): keep the subdomain across reconnects`.
- **Changelog:** add a line under "Unreleased" in [CHANGELOG.md](CHANGELOG.md) for anything users will notice.
- **Deploys:** pushes to `main` in `backend` and `landing` deploy to production, so maintainers merge those with care. A slow merge doesn't mean the change was rejected.

## License

By contributing, you agree that your contributions are licensed under the [MIT License](LICENSE).
