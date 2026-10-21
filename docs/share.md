# GoPort Share

GoPort Share provides two separate sharing modes.

## Local folder sharing

```bash
goport auth <token>
goport share ./documents
```

Options:

- `--password` securely prompts for a password without placing it in shell history.
- `--expires 2h` or `--expires 2026-10-21T12:00:00Z` closes access at a fixed time.
- `--no-preview` disables inline image, PDF, and text previews.

Local shares never upload file contents to GoPort storage. The CLI serves a read-only browser through the existing authenticated yamux tunnel and traffic counts against the account's tunnel bandwidth allowance. Every request must match the session's cryptographically random path token. A stopped share is removed from the active session map, and `/s/` paths are rejected if its subdomain is later used by a normal HTTP tunnel.

The CLI refuses filesystem roots, resolves the selected root before serving, opens files with OS-rooted traversal-resistant APIs, rejects path traversal and symlink escapes, and excludes hidden names, `.env` files, common credential names, and private-key formats. HTML, SVG, and scripts are never rendered as previews. File downloads support HTTP Range requests; folder ZIPs and file bodies stream without being loaded into memory. At most eight file or ZIP transfers run concurrently per CLI session.

## Cloud file sharing

Open `https://goport.uz/dashboard#share`, drop a supported file, choose how the link opens and when it expires, then copy the link. Public visitors do not need an account.

A link opens in one of two ways:

- **Share page** (`/s/<opaque-token>`): a GoPort page with a preview, a download button and an optional password.
- **Direct file** (`/f/<opaque-token>/<name>`, Pro): the link opens the file itself, for example a certificate or a photo, with no page around it. Direct links can't have a password. If the owner leaves Pro, the link falls back to the share page.

| | Free | Pro |
| --- | --- | --- |
| Storage | 100 MB | 5 GB |
| Largest file | 25 MB | 500 MB |
| How long files stay | 1 day by default, 3 days at most | 7 days by default, 30 days at most |
| Downloads per file | 100 | Unlimited |
| Direct file links | No | Yes |

Every complete delivery of the file counts as a download, whether it is previewed or saved. Uploads stop for everyone once all cloud files together reach `SHARE_TOTAL_STORAGE_BYTES`, which protects the server disk. Cloud uploads need a verified account with a regular (not disposable) email address, and at most `SHARE_FREE_UPLOADERS_PER_IP` different Free accounts may upload from one IP address per day.

The dashboard uploads in 8 MB chunks. Each chunk is retried on failure, and an upload interrupted for 30 minutes is discarded. The backend rejects executable and active-web formats such as HTML, JavaScript, SVG, shell scripts, and native executables. Images, plain text, and PDFs may be previewed; other formats are forced to download with `nosniff` and a restrictive content security policy.

Management endpoints require a valid PocketBase user session and enforce ownership server-side. Public identifiers use 256 bits of randomness. Lookup hashes are stored as SHA-256; the recoverable dashboard copy is encrypted with AES-GCM using `SHARE_SIGNING_KEY`. Passwords use bcrypt and successful unlocks create a short-lived HttpOnly cookie. Concurrent uploads and downloads are limited in memory.

Wrong passwords slow a visitor down: after 3 they wait 1 minute, after 5 they wait 30 minutes, and from 7 on each wrong password costs 24 hours. A visitor is one IP address (an IPv6 /64) on one link, and the count clears after a correct password or a day without mistakes. A link that collects 30 wrong passwords from all visitors within an hour pauses its password form for everyone, which stops guessing spread across many addresses. These counters live in memory, so a backend restart clears them.

Deleting a cloud share removes its storage object before deleting metadata. Revoking keeps the file and turns the link off; the owner can restore the same link until it expires. An hourly cleanup removes expired objects and records.

GoPort does not currently scan uploads for malware. Recipients should treat downloaded files as untrusted.

## Self-hosting configuration

Set these variables on the backend service:

```env
SHARE_STORAGE_DIR=./pb_data/shares
SHARE_SIGNING_KEY=replace-with-at-least-32-random-bytes
SHARE_TOTAL_STORAGE_BYTES=21474836480
SHARE_FREE_STORAGE_BYTES=104857600
SHARE_FREE_MAX_UPLOAD_BYTES=26214400
SHARE_FREE_DEFAULT_RETENTION_HOURS=24
SHARE_FREE_MAX_RETENTION_HOURS=72
SHARE_FREE_MAX_DOWNLOADS=100
SHARE_FREE_UPLOADERS_PER_IP=5
SHARE_PRO_STORAGE_BYTES=5368709120
SHARE_PRO_MAX_UPLOAD_BYTES=524288000
SHARE_PRO_DEFAULT_RETENTION_HOURS=168
SHARE_PRO_MAX_RETENTION_HOURS=720
```

The backend logs the limits it applies at startup (`share limits: ...`). `SHARE_MAX_UPLOAD_BYTES`, `SHARE_DEFAULT_RETENTION_HOURS` and `SHARE_MAX_RETENTION_HOURS` from earlier versions are no longer read.

`SHARE_STORAGE_DIR` must be private and persistent. The included Docker Compose volume already persists `/app/pb_data`, so the default directory is covered. For multi-instance deployments, place this directory on shared storage or adapt the storage service to an S3-compatible backend before adding replicas.

Keep `SHARE_SIGNING_KEY` stable across deployments. Changing it invalidates password-unlock cookies and prevents the dashboard from decrypting stored public-link tokens, although existing public links continue to resolve through their hashes.

The migration `1800316800_created_shares.go` creates a locked `shares` collection. Collection API rules remain `nil`; only the custom backend routes may read or mutate it.
