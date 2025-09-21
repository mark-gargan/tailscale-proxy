# Multi-stage build: build Go app, then run inside Tailscale container

FROM golang:1.21-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates && update-ca-certificates
COPY go.mod ./
# Copy the rest of the source
COPY . .
# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/tailscale-proxy ./cmd/tailscale-proxy

FROM ghcr.io/tailscale/tailscale:stable
# Copy binary and CA bundle (for HTTPS to Google JWKS, etc.)
COPY --from=build /out/tailscale-proxy /usr/local/bin/tailscale-proxy
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Defaults; override at runtime
ENV PORT=8080 \
    LISTEN_ADDR=":8080" \
    TS_HOSTNAME="ts-proxy" \
    TS_SERVE_MODE="tcp" \
    TS_SERVE_TCP_PORT=443

EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]

