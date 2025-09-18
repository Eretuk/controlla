FROM public.ecr.aws/docker/library/golang:1.25.1-alpine3.22 AS builder

WORKDIR /app
COPY . .

# Binary compilation
RUN go build -o controlla .

# Final minimal image
FROM public.ecr.aws/docker/library/alpine:3.22.1
WORKDIR /app

# Copy the binary and config file from the builder stage
COPY --from=builder /app/controlla /app/
COPY data/config.yaml /app/data/config.yaml

# Install bash for potential scripting needs
RUN apk add --no-cache bash

ENV SSH_KNOWN_HOSTS=/app/data/known_hosts

CMD ["./controlla", "--config-file", "/app/data/config.yaml"]