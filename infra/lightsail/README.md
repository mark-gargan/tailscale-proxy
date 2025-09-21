# Lightsail Container Service Deploy

This deploys the Docker image to an AWS Lightsail Container Service in eu-west-1 (Dublin). It supports a tailnet-only setup using Tailscale inside the container (no public endpoint).

## Prerequisites
- AWS CLI configured for `eu-west-1`
- Docker built image (or use the Dockerfile here)
- Tailscale auth key (tagged, ephemeral recommended)

## 1) Create the Container Service
- `aws lightsail create-container-service --service-name tailscale-proxy --power micro --scale 1 --region eu-west-1`

## 2) Build & Push the Image
- Build: `docker build -t ts-proxy:local ../..`
- Push to Lightsail registry:
  - `IMG=$(aws lightsail push-container-image --service-name tailscale-proxy --label app --image ts-proxy:local --region eu-west-1 --query 'image' --output text)`
  - This returns an image ref like `tailscale-proxy/app:latest` stored in Lightsail.

## 3) Prepare Deployment Spec (tailnet-only)
- Edit `infra/lightsail/containers.private.json` and replace `__IMAGE__` with `$IMG`.
- Set env values:
  - Required for Tailscale: `TS_AUTHKEY` (ephemeral), optional `TS_HOSTNAME`
  - Proxy config: `LISTEN_ADDR`, `ROUTES`, `AUTH_MODE`, etc.

## 4) Deploy
- `aws lightsail create-container-service-deployment \
    --service-name tailscale-proxy \
    --containers file://infra/lightsail/containers.private.json \
    --region eu-west-1`

The service won’t expose public ports. Access it over your tailnet. The entrypoint starts tailscaled in userspace and maps tailnet 443 (or HTTPS Serve) to the proxy.

## Optional: Public Endpoint (not using Tailscale for ingress)
- If you prefer internet ingress, open a port and set a public endpoint.
- Edit `infra/lightsail/containers.public.json` (replace `__IMAGE__`) and apply:
- `aws lightsail create-container-service-deployment \
    --service-name tailscale-proxy \
    --containers file://infra/lightsail/containers.public.json \
    --public-endpoint file://infra/lightsail/public-endpoint.json \
    --region eu-west-1`

## Rotate Keys / Update Image
- Push a new image (repeat step 2) and re-run the deployment with updated env.
- Use ephemeral `TS_AUTHKEY` and rotate regularly.

