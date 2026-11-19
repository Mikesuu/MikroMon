FROM golang:1.26-alpine AS builder
ARG VERSION=dev
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.Version=${VERSION}" -o /mikromon .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /mikromon /app/mikromon
RUN printf '#!/bin/sh\nif [ -x /app/data/mikromon.bin ] && [ -n "$(find /app/data/mikromon.bin -newer /app/mikromon 2>/dev/null)" ]; then\n  exec /app/data/mikromon.bin "$@"\nfi\nexec /app/mikromon "$@"\n' > /app/entrypoint.sh && chmod +x /app/entrypoint.sh
VOLUME ["/app/data"]
EXPOSE 8080
ENV MIKROMON_DATA=/app/data \
    MIKROMON_LISTEN=:8080 \
    MIKROMON_OTA_PATH=/app/data/mikromon.bin \
    TZ=Asia/Shanghai
ENTRYPOINT ["/app/entrypoint.sh"]
