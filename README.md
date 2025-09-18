# Controlla

**Controlla** is an automated tool for monitoring and updating Docker images in your infrastructure repositories.  
It scans your docker-compose files, checks for new image versions (using SemVer or digest fallback), notifies you via Telegram, and can automatically update tags and push changes to your git repository.

---

## Features

- Scans and parses all services and images in docker-compose files (YAML/Jinja2)
- Checks for new image versions (SemVer-aware, with digest fallback)
- Flexible filtering by keywords (e.g., `rc`, `nightly`, `test`, etc.)
- Skip list and global ignore for images (`skipped.yaml`)
- Interactive Telegram notifications (update/ignore buttons)
- Automatic tag update and git push
- Ready-to-use Docker image, CI/CD via DroneCI

---

## Quick Start

### 1. Clone the repository

```sh
git clone https://git.kolspace.cc/victor.kolomin/controlla.git
cd controlla
```

### 2. Configure

Edit `config.yaml`:

```yaml
git:
  repo_url: "git@git.kolspace.cc:victor.kolomin/test-homelab.git"
  local_repo_path: ./repos/myrepo
  branch: "main"
  ssh_key_path: "/app/id_rsa"

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
    new_tag: 2024.0.0-01-16
ignored_images:
  - ghcr.io/immich-app/postgres
```

### 4. Build and run with Docker

```sh
docker build -t controlla .
docker run --rm \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -v $(pwd)/skipped.yaml:/app/skipped.yaml \
  -v ~/.ssh/id_rsa:/app/id_rsa:ro \
  -e TELEGRAM_TOKEN=your_telegram_token \
  -e TELEGRAM_CHAT_ID=your_chat_id \
  controlla
```

---

## Telegram Bot

- Receive notifications about new image versions
- Interactive buttons: "Update", "Ignore"
- `/ignore_image <image>` command — add image to global ignore

---

## CI/CD (DroneCI)

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
      registry: git.kolspace.cc
      tags:
        - ${DRONE_TAG}
      dockerfile: Dockerfile
      context: .
      platforms: linux/amd64,linux/arm64
      username:
        from_secret: registry_user
      password:
        from_secret: registry_pass

trigger:
  event:
    - tag
```

- The Docker image will be tagged with your git tag (e.g., `v0.1.0`).

---

## Environment Variables

- `TELEGRAM_TOKEN` — Telegram bot token
- `TELEGRAM_CHAT_ID` — Chat ID for notifications
- `DOCKER_USER` / `DOCKER_PASS` — for private DockerHub access (if needed)

---

## Project Structure

```
internal/
  checker/        — service/image discovery and parsing
  config/         — configuration management
  gitrepo/        — git operations
  notifier/       — Telegram notifications
  registry/       — registry clients (dockerhub, ghcr, etc.)
  skiplist/       — skiplist and global ignore logic
  updater/        — tag update and git push
  versionutils/   — version sorting and filtering
main.go           — main service and polling logic
Dockerfile        — container build
.drone.yml        — CI/CD pipeline
config.yaml       — main config
skipped.yaml      — skiplist and ignored images
```

---

## Roadmap

- [ ] Web UI for skiplist management
- [ ] Support for other messengers
- [ ] Extended update analytics

---

## License

MIT

---

**Contacts:**   
[Git.Kolspace](https://git.kolspace.cc/victor.kolomin/controlla)