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

Open `https://goport.uz/dashboard#share`, drop a supported file, choose expiration and an optional password, then copy the generated `/s/<opaque-token>` URL. Public visitors do not need an account.

Cloud uploads default to seven days and cannot exceed the configured maximum retention. The backend rejects executable and active-web formats such as HTML, JavaScript, SVG, shell scripts, and native executables. Images, plain text, and PDFs may be previewed; other formats are forced to download with `nosniff` and a restrictive content security policy.

Management endpoints require a valid PocketBase user session and enforce ownership server-side. Public identifiers use 256 bits of randomness. Lookup hashes are stored as SHA-256; the recoverable dashboard copy is encrypted with AES-GCM using `SHARE_SIGNING_KEY`. Passwords use bcrypt and successful unlocks create a short-lived HttpOnly cookie. Password attempts, concurrent uploads, and concurrent downloads are limited in memory.

Deleting a cloud share removes its storage object before deleting metadata. An hourly cleanup removes expired objects and records. Interrupted multipart uploads are bounded by the HTTP body limit and their temporary files are removed by the request handler.

GoPort does not currently scan uploads for malware. Recipients should treat downloaded files as untrusted.

## Self-hosting configuration

Set these variables on the backend service:

```env
SHARE_STORAGE_DIR=./pb_data/shares
SHARE_SIGNING_KEY=replace-with-at-least-32-random-bytes
SHARE_MAX_UPLOAD_BYTES=104857600
SHARE_FREE_STORAGE_BYTES=262144000
SHARE_PRO_STORAGE_BYTES=5368709120
SHARE_DEFAULT_RETENTION_HOURS=168
SHARE_MAX_RETENTION_HOURS=720
```

`SHARE_STORAGE_DIR` must be private and persistent. The included Docker Compose volume already persists `/app/pb_data`, so the default directory is covered. For multi-instance deployments, place this directory on shared storage or adapt the storage service to an S3-compatible backend before adding replicas.

Keep `SHARE_SIGNING_KEY` stable across deployments. Changing it invalidates password-unlock cookies and prevents the dashboard from decrypting stored public-link tokens, although existing public links continue to resolve through their hashes.

The migration `1800316800_created_shares.go` creates a locked `shares` collection. Collection API rules remain `nil`; only the custom backend routes may read or mutate it.
