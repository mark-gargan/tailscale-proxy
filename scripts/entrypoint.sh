#!/usr/bin/env sh
set -eu

# If TS_AUTHKEY is provided, start tailscaled in userspace networking and expose the proxy.
if [ "${TS_AUTHKEY:-}" != "" ]; then
  echo "[entrypoint] Starting tailscaled (userspace) and bringing node up..."
  # Run tailscaled in background with in-memory state
  tailscaled --state=mem: --tun=userspace-networking &

  # Wait for tailscaled control socket to be ready
  i=0
  until tailscale status >/dev/null 2>&1; do
    i=$((i+1))
    if [ "$i" -gt 30 ]; then
      echo "[entrypoint] tailscaled did not become ready in time" >&2
      exit 1
    fi
    sleep 1
  done

  # Bring the node up; disable DNS management inside container
  tailscale up --authkey="${TS_AUTHKEY}" \
               --hostname="${TS_HOSTNAME:-ts-proxy}" \
               --accept-dns=false

  # Serve: map tailnet to local proxy
  case "${TS_SERVE_MODE:-tcp}" in
    https)
      echo "[entrypoint] tailscale serve https / -> http://127.0.0.1:${PORT}"
      tailscale serve https / "http://127.0.0.1:${PORT}"
      ;;
    tcp|*)
      echo "[entrypoint] tailscale serve tcp ${TS_SERVE_TCP_PORT:-443} -> 127.0.0.1:${PORT}"
      tailscale serve tcp "${TS_SERVE_TCP_PORT:-443}" "http://127.0.0.1:${PORT}"
      ;;
  esac
else
  echo "[entrypoint] TS_AUTHKEY not set; running without Tailscale."
fi

echo "[entrypoint] Starting tailscale-proxy on ${LISTEN_ADDR:-:8080}"
exec tailscale-proxy

