# Dockdater

Keep Docker Compose containers up-to-date automatically.

Dockdater watches all running containers on a Docker daemon and recreates those tagged with `dockdater.enabled=true` whenever a newer image is available for their tag (`latest`, `3`, `1.22-alpine`, etc).

---

## How it works

1. **Discovery** — scans every container on the daemon, filtering to those with label `dockdater.enabled=true`.
2. **Update check** — pulls the image by tag and compares the local digest with the freshly-pulled digest.
3. **Recreate** — if the digest changed, stops/removes the old container and creates a new one with the same config but the new image.
4. **Cleanup** — removes the old dangling image (unless still in use by another container).

---

## Quick start

### 1. Tag a container

In any `docker-compose.yml` or `compose.yml`:

```yaml
services:
  nginx:
    image: nginx:latest
    labels:
      dockdater.enabled: "true"
```

> **Note:** The value must be the **string** `"true"`.

### 2. Run Dockdater

#### Binary

```bash
# build
go build .

# run
./dockdater -interval 5m
```

#### Docker

```bash
docker build -t dockdater .
docker run -d \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e DOCKDATER_INTERVAL=10m \
  --name dockdater \
  dockdater
```

#### Docker Compose (self-hosting)

```yaml
services:
  dockdater:
    image: dockdater
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      DOCKDATER_INTERVAL: "10m"
```

---

## Configuration

| Setting | Environment variable | CLI flag | Default |
|---|---|---|---|
| Check interval | `DOCKDATER_INTERVAL` | `-interval` | `5m` |

* The interval accepts any duration string (`1h`, `30m`, `10s`, `24h`).
* The CLI flag overrides the environment variable.

---

## Requirements

* Docker Engine accessible via `/var/run/docker.sock` (or `DOCKER_HOST`).
* The daemon must expose the Engine API — Dockdater talks directly through the SDK.

---

## What gets recreated?

Only containers with the label:

```yaml
labels:
  - "dockdater.enabled=true"
```

Dockdater determines each container's project and service name from the standard Docker Compose labels (`com.docker.compose.project`, `com.docker.compose.service`) already present on compose-launched containers. No compose file path needs to be supplied.

