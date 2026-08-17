# modship

A web-based tool to automate deployment of Minecraft modpack servers with Docker, [mc-router](https://github.com/itzg/mc-router), and Cloudflare DNS.

## How it works

1. **Search** for a modpack on CurseForge or Modrinth via the web UI
2. **Pick a version** — modship resolves the file ID / version ID
3. **Deploy** — modship generates a per-server `docker-compose.yml`, runs `docker compose up`, registers the hostname with mc-router, and creates a Cloudflare DNS record
4. **Manage** — start, stop, or delete servers from the UI (delete cleans up containers, DNS, files, and DB state)

Each server runs as its own Docker Compose project (`docker compose -p <name>`), isolated on a shared external network. The [itzg/minecraft-server](https://github.com/itzg/docker-minecraft-server) image handles modpack download and installation automatically via env vars — modship doesn't download modpacks itself.

**Container topology:**
```
client → mc-router (:25565, routes by hostname)
            ├─ server-a → minecraft-server-a
            └─ server-b → minecraft-server-b
```

## Prerequisites

- Docker + Docker Compose installed on the host
- A running `mc-router` container with HTTP API enabled (port 25566)
- A shared Docker network (default: `modship_minecraft-network`)
- CurseForge API key (for CurseForge modpacks)
- Cloudflare API token + zone ID (for DNS management)

## Setup

### 1. Create the shared network and mc-router

```bash
docker network create modship_minecraft-network

docker run -d --name mc-router \
  --network modship_minecraft-network \
  -p 25565:25565 \
  -p 25566:25566 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  ghcr.io/itzg/mc-router:latest \
  -in-docker -api :25566
```

### 2. Configure environment

```bash
export CF_API_KEY=your_curseforge_api_key
export CLOUDFLARE_API_KEY=your_cloudflare_api_token
export CLOUDFLARE_ZONE_ID=your_zone_id
export MODSHIP_HOST_IP=your.server.public.ip
export MODSHIP_BASE_DOMAIN=example.com   # default: max4king.com
```

### 3. Run modship

```bash
go run ./cmd/modship
# → open http://localhost:8080
```

## Configuration

All settings via environment variables:

| Variable | Default | Description |
|---|---|---|
| `CF_API_KEY` | — | CurseForge API key |
| `CLOUDFLARE_API_KEY` | — | Cloudflare API token |
| `CLOUDFLARE_ZONE_ID` | — | Cloudflare DNS zone ID |
| `MODSHIP_HOST_IP` | — | Host public IP for DNS records |
| `MODSHIP_BASE_DOMAIN` | `max4king.com` | Parent domain for server subdomains |
| `MODSHIP_LISTEN` | `:8080` | HTTP listen address |
| `MODSHIP_DATA_DIR` | `./deployments` | Where compose files + server data live |
| `MODSHIP_DB_PATH` | `<data_dir>/modship.db` | SQLite database path |
| `MODSHIP_ROUTER_URL` | `http://localhost:25566` | mc-router HTTP API URL |
| `MODSHIP_DOCKER_NETWORK` | `modship_minecraft-network` | Shared Docker network name |

## API

| Method | Path | Description |
|---|---|---|
| `GET` | `/` | Web UI |
| `GET` | `/api/search?q=&provider=` | Search modpacks |
| `GET` | `/api/versions?slug=&provider=` | List modpack versions |
| `POST` | `/api/servers` | Deploy a new server |
| `POST` | `/api/servers/{id}/start` | Start a stopped server |
| `POST` | `/api/servers/{id}/stop` | Stop a running server |
| `DELETE` | `/api/servers/{id}` | Delete a server entirely |
