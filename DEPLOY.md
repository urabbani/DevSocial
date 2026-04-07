# Deployment

## VPS

- **Provider:** DigitalOcean (or any VPS)
- **SSH:** `ssh -i <key> <user>@<host>`

## Service

KarpathyTalk runs as a systemd service:

```bash
systemctl {start,stop,restart,status} karpathytalk
```

- **Binary:** `/opt/karpathytalk/karpathytalk`
- **Data:** `/var/lib/karpathytalk/` (SQLite DB + uploads)
- **Config:** `/etc/karpathytalk.env` (secrets, not in repo)
- **Port:** 8080 (behind Caddy on 80/443)
- **Domain:** `https://karpathytalk.com`
- **TLS:** Automatic via Caddy + Let's Encrypt
- **Caddy config:** `/etc/caddy/Caddyfile`
- **Caddy service:** `systemctl {start,stop,restart,status} caddy`

## Deploying updates

```bash
# Build for Linux from macOS
GOOS=linux GOARCH=amd64 go build -o karpathytalk ./cmd/karpathytalk

# Stop, upload, start (binary can't be overwritten while running)
ssh <user>@<host> 'systemctl stop karpathytalk'
scp karpathytalk <user>@<host>:/opt/karpathytalk/
scp -r templates static schema.sql <user>@<host>:/opt/karpathytalk/
ssh <user>@<host> 'systemctl start karpathytalk'
```

## Environment variables

Set in `/etc/karpathytalk.env` (mode 600):

```
GITHUB_CLIENT_ID=...
GITHUB_CLIENT_SECRET=...
BASE_URL=https://karpathytalk.com
```
