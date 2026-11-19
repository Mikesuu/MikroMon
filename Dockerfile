FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /mikromon .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /mikromon /app/mikromon
VOLUME ["/app/data"]
EXPOSE 8080
ENV MIKROMON_DATA=/app/data \
    MIKROMON_LISTEN=:8080 \
    TZ=Asia/Shanghai
ENTRYPOINT ["/app/mikromon"]
