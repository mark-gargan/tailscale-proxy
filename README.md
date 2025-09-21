# Tailscale Proxy — Setup Guide

This repo provides a small, secure reverse proxy configurable via `.env`, with optional auth (shared secret or Google OIDC), multi-route support, and AWS Terraform deployment.

## 1) Prerequisites
- Go 1.21+ (for local dev) and Make
- Docker (for container builds)
- Terraform 1.3+ and AWS CLI (configured for eu-west-1)
- Optional: Tailscale CLI if exposing via your existing tailnet

## 2) Configure .env
- Copy the sample: `cp .env.sample .env`
- Minimal example:
  - `LISTEN_ADDR=":8080"`
  - `ROUTES=/api=http://localhost:9000,/app=http://localhost:3000`
  - Auth (pick one):
    - Shared secret: `AUTH_MODE=shared_secret`, `AUTH_SHARED_SECRET=change-me`
    - Google OIDC: `AUTH_MODE=google`, `GOOGLE_OIDC_CLIENT_ID=YOUR_CLIENT_ID`
      - Optional allowlists: `ALLOWED_EMAILS=you@example.com`, `ALLOWED_DOMAINS=example.com`
- Optional: `DEFAULT_BACKEND=http://localhost:8081`, `PRESERVE_HOST=false`

## 3) Run Locally
- Start: `make run`
- Test:
  - Shared secret: `curl -H 'Authorization: Bearer change-me' http://localhost:8080/api/health`
  - Google OIDC: send a Google `id_token` as Bearer or `id_token` cookie
- Logs show requests, status codes, and durations

## 4) Use with Tailscale (optional)
- Keep proxy bound locally (e.g., `LISTEN_ADDR=127.0.0.1:8080`)
- Expose via Tailscale on your host: `tailscale serve tcp 8080 localhost:8080`
- Rely on Tailscale ACLs + this proxy’s auth for layered security

## 5) Docker Image
- Build: `docker build -t tailscale-proxy:local .`
- Run: `docker run --rm -p 8080:8080 --env-file .env tailscale-proxy:local`

## 6) Deploy to AWS (Terraform, eu-west-1 Dublin)
- `cd infra/terraform && terraform init`
- Apply base infra: `terraform apply -var "project_name=tailscale-proxy"`
- Build and push image:
  - `ECR=$(terraform output -raw ecr_repository_url)`
  - `aws ecr get-login-password --region eu-west-1 | docker login --username AWS --password-stdin ${ECR%/*}`
  - `docker build -t $ECR:latest ../../ && docker push $ECR:latest`
- Roll out: `terraform apply -var "project_name=tailscale-proxy" -var "force_new_deployment=true"`
- Pass env/secrets:
  - Env: `-var 'env_vars={LISTEN_ADDR=":8080",AUTH_MODE="shared_secret"}'`
  - Secrets (SSM/Secrets Manager ARN): `-var 'secret_arns={AUTH_SHARED_SECRET="arn:aws:ssm:eu-west-1:..."}'`
- Optional TLS: add ACM cert in eu-west-1
  - `-var 'certificate_arn=arn:aws:acm:eu-west-1:...:certificate/...'`
- Output ALB URL: `terraform output alb_dns_name` (CNAME your domain if desired)

## 7) Google OIDC Notes
- Create an OAuth 2.0 Client ID (Web) in Google Cloud Console; use the Client ID in `GOOGLE_OIDC_CLIENT_ID`
- Obtain an `id_token` from your frontend/app and send as Bearer or `id_token` cookie
- Optionally restrict by `ALLOWED_EMAILS`/`ALLOWED_DOMAINS`

## 8) Troubleshooting
- 401: check `AUTH_MODE` and token/secret
- 502: backend unreachable; verify `ROUTES` targets and security groups
- Zero-downtime updates: push image, set `force_new_deployment=true`
- Logs: CloudWatch `/ecs/tailscale-proxy` (AWS) or stdout locally

Security tip: never commit secrets; prefer AWS SSM/Secrets Manager in prod.
