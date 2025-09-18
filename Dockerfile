FROM public.ecr.aws/docker/library/golang:1.25.1-alpine3.22 AS builder

WORKDIR /app
COPY . .

# Binary compilation
RUN go build -o controlla .

# Final minimal image
FROM public.ecr.aws/docker/library/alpine:3.22.1
WORKDIR /app

# Копируем бинарник и всё нужное (например, config.yaml, skipped.yaml)
COPY --from=builder /app/controlla /app/
COPY config.yaml skipped.yaml /app/

# Если нужны ssh-ключи или другие файлы — добавь COPY

# Установи bash, если нужен (например, для скриптов)
RUN apk add --no-cache bash

CMD ["./controlla"]