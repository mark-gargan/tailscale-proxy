#!/usr/bin/env bash
set -euo pipefail

# Deploy the container to AWS Lightsail Container Service.
# - Builds the Docker image
# - Pushes to Lightsail registry
# - Substitutes the image into the chosen spec (private/public)
# - Creates the service (if missing) and deploys
#
# Usage examples:
#   bash scripts/deploy_lightsail.sh
#   bash scripts/deploy_lightsail.sh --mode private --service tailscale-proxy --region eu-west-1
#   bash scripts/deploy_lightsail.sh --mode public --service tailscale-proxy --region eu-west-1

MODE="private"           # private|public
SERVICE_NAME="tailscale-proxy"
REGION="eu-west-1"
IMAGE_LABEL="app"
LOCAL_TAG="ts-proxy:local"
POWER="micro"
SCALE="1"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="$2"; shift 2;;
    --service) SERVICE_NAME="$2"; shift 2;;
    --region) REGION="$2"; shift 2;;
    --label) IMAGE_LABEL="$2"; shift 2;;
    --local-tag) LOCAL_TAG="$2"; shift 2;;
    --power) POWER="$2"; shift 2;;
    --scale) SCALE="$2"; shift 2;;
    -h|--help)
      echo "Usage: $0 [--mode private|public] [--service NAME] [--region REGION] [--label LABEL] [--local-tag TAG] [--power SIZE] [--scale N]"; exit 0;;
    *) echo "Unknown arg: $1"; exit 1;;
  esac
done

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
SPEC_DIR="$ROOT_DIR/infra/lightsail"

if ! command -v aws >/dev/null; then
  echo "aws CLI is required" >&2; exit 1
fi
if ! command -v docker >/dev/null; then
  echo "docker is required" >&2; exit 1
fi

echo "[lightsail] Ensuring service exists: $SERVICE_NAME ($REGION)"
if ! aws lightsail get-container-services --service-name "$SERVICE_NAME" --region "$REGION" >/dev/null 2>&1; then
  aws lightsail create-container-service \
    --service-name "$SERVICE_NAME" \
    --power "$POWER" \
    --scale "$SCALE" \
    --region "$REGION"
fi

echo "[lightsail] Building image: $LOCAL_TAG"
docker build -t "$LOCAL_TAG" "$ROOT_DIR"

echo "[lightsail] Pushing image to Lightsail registry with label: $IMAGE_LABEL"
IMG=$(aws lightsail push-container-image \
  --service-name "$SERVICE_NAME" \
  --label "$IMAGE_LABEL" \
  --image "$LOCAL_TAG" \
  --region "$REGION" \
  --query 'image' --output text)

echo "[lightsail] Image reference: $IMG"

case "$MODE" in
  private) SPEC_SRC="$SPEC_DIR/containers.private.json" ;;
  public)  SPEC_SRC="$SPEC_DIR/containers.public.json"  ;;
  *) echo "Invalid --mode: $MODE (expected private|public)" >&2; exit 1 ;;
esac

if [[ ! -f "$SPEC_SRC" ]]; then
  echo "Spec not found: $SPEC_SRC" >&2; exit 1
fi

SPEC_TMP=$(mktemp)
sed "s#__IMAGE__#$IMG#g" "$SPEC_SRC" > "$SPEC_TMP"

DEPLOY_ARGS=(--service-name "$SERVICE_NAME" --containers "file://$SPEC_TMP" --region "$REGION")
if [[ "$MODE" == "public" ]]; then
  DEPLOY_ARGS+=(--public-endpoint "file://$SPEC_DIR/public-endpoint.json")
fi

echo "[lightsail] Deploying ($MODE) ..."
aws lightsail create-container-service-deployment "${DEPLOY_ARGS[@]}"

rm -f "$SPEC_TMP"
echo "[lightsail] Done. Check service status: aws lightsail get-container-services --service-name $SERVICE_NAME --region $REGION"

