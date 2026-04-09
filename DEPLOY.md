# Deployment

## VPS

- **Provider:** DigitalOcean (or any VPS)
- **SSH:** `ssh -i <key> <user>@<host>`

## Service

DevSocial runs as a systemd service:

```bash
systemctl {start,stop,restart,status} devsocial
```

- **Binary:** `/opt/devsocial/devsocial`
- **Data:** `/var/lib/devsocial/` (SQLite DB + uploads)
- **Config:** `/etc/devsocial.env` (secrets, not in repo)
- **Port:** 8080 (behind Caddy on 80/443)
- **Domain:** `https://devsocial.app`
- **TLS:** Automatic via Caddy + Let's Encrypt
- **Caddy config:** `/etc/caddy/Caddyfile`
- **Caddy service:** `systemctl {start,stop,restart,status} caddy`

## Deploying updates

```bash
# Build for Linux from macOS
GOOS=linux GOARCH=amd64 go build -o devsocial ./cmd/devsocial

# Stop, upload, start (binary can't be overwritten while running)
ssh <user>@<host> 'systemctl stop devsocial'
scp devsocial <user>@<host>:/opt/devsocial/
scp -r templates static schema.sql <user>@<host>:/opt/devsocial/
ssh <user>@<host> 'systemctl start devsocial'
```

## Environment variables

Set in `/etc/devsocial.env` (mode 600):

```
GITHUB_CLIENT_ID=...
GITHUB_CLIENT_SECRET=...
BASE_URL=https://devsocial.app
```
