# Controlla

**Controlla** — это инструмент для автоматического мониторинга и обновления Docker-образов в ваших инфраструктурных репозиториях.  
Он анализирует ваши docker-compose файлы, проверяет наличие новых версий образов, уведомляет через Telegram и может автоматически обновлять теги и пушить изменения в git.

---

## Возможности

- Поиск и анализ всех сервисов и образов в docker-compose файлах
- Проверка наличия новых версий образов (SemVer и digest fallback)
- Гибкая фильтрация по ключевым словам (например, `rc`, `nightly`, `test`)
- Скип-лист и глобальный ignore для образов (через `skipped.yaml`)
- Интерактивные уведомления в Telegram (обновить/игнорировать)
- Автоматическое обновление тегов и пуш в git-репозиторий
- Готовый Docker-образ, CI/CD через DroneCI

---

## Быстрый старт

### 1. Клонируйте репозиторий

```sh
git clone https://git.kolspace.cc/victor.kolomin/controlla.git
cd controlla
```

### 2. Настройте конфиг

Отредактируйте `config.yaml`:

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

### 3. Заполните `skipped.yaml` (опционально)

```yaml
updates:
  - image: ghcr.io/lldap/lldap
    new_tag: 2024.0.0-01-16
ignored_images:
  - ghcr.io/immich-app/postgres
```

### 4. Соберите и запустите через Docker

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

## Telegram-бот

- Получайте уведомления о новых версиях образов
- Интерактивные кнопки: "Обновить", "Игнорировать"
- Команда `/ignore_image <image>` — добавить образ в глобальный ignore

---

## CI/CD (DroneCI)

Пример `.drone.yml` для автоматической сборки и публикации Docker-образа по git-тегу:

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

---

## Переменные окружения

- `TELEGRAM_TOKEN` — токен Telegram-бота
- `TELEGRAM_CHAT_ID` — ID чата для уведомлений
- `DOCKER_USER` / `DOCKER_PASS` — если нужен приватный доступ к DockerHub

---

## Структура проекта

```
internal/
  checker/        — поиск и парсинг сервисов
  config/         — работа с конфигом
  gitrepo/        — git-операции
  notifier/       — Telegram-уведомления
  registry/       — работа с реестрами образов (dockerhub, ghcr, quay и др.)
  skiplist/       — skiplist и глобальный ignore
  updater/        — обновление тегов и git push
  versionutils/   — сортировка и фильтрация версий
main.go           — основной сервис и логика опроса
Dockerfile        — сборка контейнера
.drone.yml        — CI/CD pipeline
config.yaml       — основной конфиг
skipped.yaml      — skiplist и ignore-образы
```

---

## TODO / Roadmap

- [ ] Web-интерфейс для управления skiplist
- [ ] Поддержка других мессенджеров
- [ ] Расширенная аналитика по обновлениям

---

## Лицензия

MIT

---

**Контакты:**  
[Telegram](https://t.me/yourusername)  
[Git.Kolspace](https://git.kolspace.cc/victor.kolomin/controlla)
