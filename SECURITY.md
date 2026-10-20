# Security policy

## Supported versions

GoPort is maintained by a small team, so security fixes go into the **latest release only**. Please upgrade before reporting, and check `goport --version`.

| Component | Supported |
|---|---|
| `goport` CLI | The latest release on the [releases page](https://github.com/muhammad-deve/GoPort/releases/latest). Older 0.1.x versions don't get fixes. |
| Go library (`github.com/muhammad-deve/goport`) | The latest tagged version |
| Tunnel server and API (`backend/`) | The `main` branch, which the hosted service at goport.uz runs |
| Website and dashboard (`landing/`) | The `main` branch, deployed at goport.uz |

## Reporting a vulnerability

**Please don't open a public issue for a security problem.**

1. **Preferred:** use GitHub's private reporting. Open the [Security tab](https://github.com/muhammad-deve/GoPort/security) of this repository and choose **Report a vulnerability**.
2. **If that button isn't there**, email **support@contact.goport.uz** with "SECURITY" in the subject.

Include:

- which component is affected (CLI, server, website/dashboard, hosted service) and the version or commit
- the steps to reproduce, and a proof of concept if you have one
- what an attacker gains
- whether you've told anyone else

## What to expect

- We confirm receipt within **7 days**.
- Within **14 days** we tell you whether we consider it a vulnerability and what we plan to do.
- We aim to release a fix within **90 days** and keep you updated if it takes longer.
- We publish an advisory when the fix ships. If you want credit, tell us how to name you.

Please give us a chance to fix the problem before you disclose it publicly. If we stop responding or miss the 90 days without explanation, you may disclose.

## Scope

In scope:

- the `goport` CLI and its local request inspector
- the tunnel server and API in `backend/`
- the website and account dashboard at goport.uz
- the hosted service: `goport.uz`, `back.goport.uz`, and tunnel subdomains, as far as GoPort's own code is concerned

Out of scope:

- vulnerabilities in the applications people expose through a tunnel
- volumetric denial of service, spam, and social engineering
- third-party services such as Lemon Squeezy, Google sign-in, and email delivery. Report those to the vendor.
- findings that need a compromised machine or a malicious local administrator
- missing headers or best-practice notes with no demonstrated impact

## Good-faith research

We won't take action against research that follows these rules:

- Use only your own account and your own tunnels.
- Don't access, change, or keep other people's data. If you reach some by accident, stop and tell us.
- Don't degrade the service for others. No load testing against goport.uz.
- Give us reasonable time to fix the problem before you disclose it.

## Known limitations

The current design has some limits you should know about before you rely on GoPort. The biggest is this: **the connection between the CLI and the tunnel server is plain TCP, not TLS.** [docs/security-model.md](docs/security-model.md) explains:

- what each connection protects
- what the server can see and log
- how the request inspector behaves
- the precautions we recommend
