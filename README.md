# Controlla

**Controlla** is an automated tool for monitoring and updating Docker images in your infrastructure repositories.  
It scans your docker-compose files, checks for new image versions (using SemVer or digest fallback), notifies you via Telegram, and can automatically update tags and push changes to your git repository.

---

## Features

- Scans and parses all services and images in docker-compose files (YAML)
- Checks for new image versions (SemVer-aware, with digest fallback)
- Flexible filtering by keywords (e.g., `rc`, `nightly`, `test`, etc.)
- Skip list and global ignore for images (`skipped.yaml`)
- Interactive Telegram notifications (update/ignore buttons)
- Automatic tag update and git push
- Ready-to-use Docker image, CI/CD via DroneCI / Woodpecker

---

## Quick Start

### 1. Clone the repository

```sh
git clone https://git.kolspace.cc/victor.kolomin/controlla.git
cd controlla
```

### 2. Configure

Edit `data/config.yaml`:

```yaml
git:
  repo_url: "git@git.kolspace.cc:youruser/homelab.git"
  local_repo_path: ./data/repos/myrepo
  branch: "main"
  ssh_key_path: "/app/data/id_rsa"
  author_name: "Controlla Bot"
  author_email: "bot@controlla.local"

# Optional: GitHub Personal Access Token for authenticated ghcr.io requests.
# Significantly reduces the chance of hitting 429 rate limits.
# Can also be set via the CONTROLLA__GITHUB__TOKEN environment variable.
# github_token: "ghp_yourTokenHere"

checker:
  watch_folder: services
  proposed_tags_count: 3
  interval_minutes: 15
  skip_list_file: "skipped.yaml"
  ignore_keywords:
    - rc
    - alpha
    - nightly
    - test
    - debug
    - arm64v8
    - arm32v7
    - ubuntu
```

### 3. Fill in `skipped.yaml` (optional)

```yaml
updates:
  - image: ghcr.io/lldap/lldap
    new_tag: v2.0.0
ignored_images:
  - ghcr.io/immich-app/postgres
```

### 4. Build and run with Docker

```sh
docker build -t controlla .
docker run --rm \
  -v $(pwd)/data/config.yaml:/app/data/config.yaml \
  -v $(pwd)/data/skipped.yaml:/app/data/skipped.yaml \
  -v ~/.ssh/id_rsa:/app/data/id_rsa:ro \
  -e CONTROLLA__TELEGRAM__TOKEN=your_telegram_token \
  -e CONTROLLA__TELEGRAM__CHAT__ID=your_chat_id \
  controlla
```

---

## Telegram Bot

- Receive notifications about new image versions
- Interactive buttons: "Update", "Ignore"
- `/ignore_image <image>` command — add image to global ignore list

---

## Environment Variables

| Variable | Description |
|---|---|
| `CONTROLLA__TELEGRAM__TOKEN` | Telegram bot token |
| `CONTROLLA__TELEGRAM__CHAT__ID` | Chat ID for notifications |
| `CONTROLLA__DOCKERHUB__USERNAME` | DockerHub username (for private images) |
| `CONTROLLA__DOCKERHUB__PASSWORD` | DockerHub password / token |
| `CONTROLLA__GITHUB__TOKEN` | GitHub PAT for authenticated ghcr.io requests (recommended) |
| `SSH_KEY_PATH` | Path to SSH key inside the container (default: `/app/data/id_rsa`) |
| `SSH_KNOWN_HOSTS` | Path to known_hosts inside the container (default: `/app/data/known_hosts`) |

---

## Fixing 429 Rate Limit Errors on ghcr.io

GHCR enforces strict rate limits for anonymous requests. If you see:

```
Error getting tags for ghcr.io/linuxserver/bookstack: GHCR request failed with status code: 429
```

Do the following:

1. **Create a GitHub Personal Access Token (PAT)** with the `read:packages` scope:  
   `GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens`

2. **Pass the token** via environment variable:
   ```sh
   -e CONTROLLA__GITHUB__TOKEN=ghp_yourTokenHere
   ```
   or add `github_token: "ghp_yourTokenHere"` to `config.yaml`.

Controlla also implements **automatic retry with backoff**: on a 429 response it reads the `Retry-After` header and waits accordingly (up to 3 retries). If the header is absent, exponential backoff is used: 10 s → 20 s → 40 s.

---

## CI/CD (DroneCI / Woodpecker)

Example `.drone.yml` for automatic Docker image build and push on git tag:

```yaml
kind: pipeline
type: docker
name: build-control-container

steps:
  - name: build and push
    image: plugins/docker
    settings:
      repo: registry.example.com/youruser/controlla
      registry: registry.example.com
      tags:
        - ${DRONE_TAG}
      dockerfile: Dockerfile
      context: .
      username:
        from_secret: registry_user
      password:
        from_secret: registry_pass

trigger:
  event:
    - tag
```

---

## Project Structure

```
internal/
  checker/        — docker-compose service/image discovery and parsing
  config/         — configuration loading (YAML + env overrides)
  gitrepo/        — git operations (clone, pull, push)
  notifier/       — Telegram notifications
  registry/       — registry clients (dockerhub, ghcr, common)
  skiplist/       — skip list and global ignore logic
  updater/        — tag update and git push
  versionutils/   — SemVer sorting and filtering
main.go           — main service loop and Telegram handler
Dockerfile        — container build
data/config.yaml  — main configuration file
data/skipped.yaml — skip list and ignored images
```

---

## Changelog

### Recent improvements

#### GHCR 429 rate limit fix
- Added **retry with backoff** for all ghcr.io requests: on a 429 response the `Retry-After` header is respected; exponential backoff (10 s → 20 s → 40 s) is used as a fallback. Up to 3 retries.
- Added **GitHub Personal Access Token** support (`github_token` / `CONTROLLA__GITHUB__TOKEN`). Authenticated token requests to ghcr.io have a much higher rate limit than anonymous ones.
- `GetDigest` on the GHCR client now also uses retry logic and proper authentication.

#### HTTP client stability
- Added a **15-second timeout** to `DockerHubClient` and `GhcrClient` — both previously had no timeout (only `CommonRegistryClient` did).

#### Reduced registry pressure
- Added a **2-second delay between image checks** in the main polling loop to avoid sending all requests in a burst.

#### Data race fix
- Added mutex locking to `skiplist.IsImageIgnored` — it was the only method that read shared data without holding the lock.

#### Dependency consistency
- `sort.go` and `main.go` now both use `github.com/Masterminds/semver/v3`. Previously v1 and v3 were mixed in the same binary.

---

## Roadmap

- [ ] Web UI for skip list management
- [ ] Support for other messengers
- [ ] Extended update analytics

---

## License

MIT

---

**Contacts:**  
[Git.Kolspace](https://git.kolspace.cc/victor.kolomin/controlla)
