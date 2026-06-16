<div align="center">

<img src="docs/goport-logo.svg" alt="GoPort" width="420" />


**Expose localhost. Instantly.**

Make your local projects accessible from anywhere with a secure public URL — all with a single command.

[![Go](https://img.shields.io/badge/Go-1.23-00ADD8.svg?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-30f294.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-00c8ff.svg)](#contributing)

</div>

---

## What is GoPort?

GoPort is an open-source alternative to ngrok and jprq that turns local applications into publicly accessible HTTPS endpoints.

Whether you're testing webhooks, sharing a demo, or exposing an API, GoPort gives your localhost a public URL in seconds — without port forwarding, firewall changes, or a public IP.

Built for developers, GoPort makes it easy to expose services running on your machine while keeping setup simple and infrastructure under your control. It uses a persistent multiplexed connection to efficiently route traffic between the public internet and your local application.

Unlike many hosted tunneling services, GoPort can be fully self-hosted, allowing teams to manage their own tunnels, domains, and infrastructure without vendor lock-in.


## How it works

1. **Authenticate the CLI**

   ```bash
   goport auth <token>
   ```

   Authenticate the CLI with your GoPort account and connect it to your server.

2. **Start a tunnel**

   ```bash
   goport http 8080
   ```

   GoPort opens a secure outbound connection and instantly creates a public HTTPS URL for your local application.

3. **Use a custom subdomain**

   ```bash
   goport http 8080 --custom myapp
   ```

   Request a memorable URL for your tunnel:

   ```txt
   https://myapp.goport.uz
   ```

4. **Generate a new URL**

   ```bash
   goport http 8080 --reset
   ```

   Discards the current subdomain and creates a brand-new public URL.

5. **Monitor traffic in real time**

   After connecting, GoPort launches a local dashboard where you can inspect incoming requests, responses, tunnel status, and latency.

   <div align="center">
     <img src="docs/terminal-preview.svg" alt="GoPort Dashboard" width="800" />
   </div>

6. **Route traffic to localhost**

   When someone visits your public URL, GoPort forwards the request through a secure tunnel directly to your local application.

7. **Disconnect cleanly**

   Press `Ctrl+C` at any time to close the tunnel. GoPort automatically removes the active session and releases the URL.

<div align="center">

</div>

## Installation

### <img src="https://cdn.jsdelivr.net/gh/devicons/devicon/icons/apple/apple-original.svg" width="18" height="18" /> macOS

Install via Homebrew:

```bash
brew tap muhammad-deve/goport
brew install goport
```

### <img src="https://cdn.jsdelivr.net/gh/devicons/devicon/icons/windows11/windows11-original.svg" width="18" height="18" /> Windows

Install via Chocolatey:

```bash
choco install goport
```

### <img src="https://cdn.jsdelivr.net/gh/devicons/devicon/icons/linux/linux-original.svg" width="18" height="18" /> Linux

Install with the official installation script:

```bash
curl -fsSL https://github.com/muhammad-deve/GoPort/releases/latest/download/goport-linux-amd64 -o /usr/local/bin/goport && chmod +x /usr/local/bin/goport
```

### Verify Installation

```bash
goport --version
```

You should see the installed GoPort version printed to the terminal.

---

## Technical Details

### Why TCP + yamux instead of a plain HTTP proxy?

GoPort uses a persistent TCP connection between the CLI and the server, with request multiplexing powered by yamux.

A plain HTTP proxy opens a new connection per request and struggles with protocols that require long-lived connections, such as WebSockets and streaming responses. By multiplexing multiple streams over a single TCP connection, GoPort handles concurrency efficiently while remaining reliable behind NAT and firewalls.

The CLI also detects HTTP upgrade requests and switches to raw bidirectional forwarding when necessary, allowing WebSocket-based applications to work seamlessly.

One additional benefit is that GoPort preserves the original public `Host` header when forwarding requests to your local application. This keeps tools such as Swagger UI, OpenAPI, and host-based routing frameworks working exactly as they would in production.

## Tech Stack

- **[Go](https://go.dev)** — CLI and tunnel engine
- **[Cobra](https://github.com/spf13/cobra)** — Command-line interface
- **[yamux](https://github.com/hashicorp/yamux)** — TCP stream multiplexing
- **[PocketBase](https://pocketbase.io)** — Authentication, tunnel management, and subdomain registry
- **[Next.js](https://nextjs.org)** — Website and dashboard

## License

GoPort is released under the [MIT License](LICENSE).
