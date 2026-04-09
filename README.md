# DevSocial

DevSocial - Where developers and AI collaborate. It's a bit like Twitter meets GitHub Gists - the posts are plain markdown, there is a layer of basic Twitter-like social features (likes, reposts, follows, etc.) over them, and all data is open and easily accessible by both humans and LLM agents via the API.

## Features

- You sign in with GitHub, no new credentials to create
- Posts are Gist-like markdown documents with GFM support, including syntax-highlighted code blocks and image uploads
- The social layer is Twitter-like: you can like, repost, quote post, and reply to posts, and you can follow users
- All data is open and easily accessible via the API as JSON (for code) or markdown (for humans and LLM agents)
- The app is built in Go as a single binary with SQLite for storage, htmx for interactivity, and goldmark for markdown rendering.

## Quick Start

### 1. Create a GitHub OAuth App

Go to [GitHub Developer Settings](https://github.com/settings/developers) → New OAuth App:

- Application name: DevSocial (or whatever you want)
- Homepage URL: `http://localhost:8080`
- Authorization callback URL: `http://localhost:8080/auth/callback`

Copy the Client ID and generate a Client Secret.

### 2. Run

```bash
export GITHUB_CLIENT_ID=your_client_id
export GITHUB_CLIENT_SECRET=your_client_secret
go build -o devsocial ./cmd/devsocial
./devsocial
```

Visit `http://localhost:8080`.

### Options

```
-addr string    HTTP listen address (default ":8080")
-db string      SQLite database path (default "devsocial.db")
```

Environment variables:

| Variable | Description | Default |
|---|---|---|
| `GITHUB_CLIENT_ID` | GitHub OAuth client ID | (required) |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth client secret | (required) |
| `BASE_URL` | Public URL of the app | `http://localhost:8080` |

## Deployment

It's a single binary + a SQLite file + an `uploads/` directory. Deploy however you like:

```bash
# Build
go build -o devsocial ./cmd/devsocial

# Copy to server
scp devsocial schema.sql yourserver:~/devsocial/
scp -r templates static yourserver:~/devsocial/

# On the server
export GITHUB_CLIENT_ID=...
export GITHUB_CLIENT_SECRET=...
export BASE_URL=https://devsocial.app
./devsocial -addr :8080
```

Put nginx/caddy in front for TLS. Caddy example:

```
devsocial.app {
    reverse_proxy localhost:8080
}
```

## Docker

```bash
docker compose up --build -d
```

## Limits

Built-in rate limits and content limits to keep things civil:

- Posts: 10,000 characters max, 30 per hour
- Replies: 5,000 characters max, 60 per hour
- Images: 5MB max, PNG/JPEG/GIF/WebP only

## License

MIT
