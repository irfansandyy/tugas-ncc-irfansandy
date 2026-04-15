#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODEL_REF="${1:-hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF:Q6_K}"
USE_HOST_NGINX="${USE_HOST_NGINX:-1}"

cd "${ROOT_DIR}"

if [[ ! -f .env ]]; then
  cp .env.example .env
fi

./scripts/docker-model-run.sh "${MODEL_REF}"

if [[ "${USE_HOST_NGINX}" == "1" ]]; then
  docker compose --env-file .env up -d --build db backend frontend
  echo "Stack is running behind host Nginx mode."
  echo "Frontend upstream: http://127.0.0.1:${FRONTEND_HOST_PORT:-3000}"
  echo "Backend upstream:  http://127.0.0.1:${BACKEND_HOST_PORT:-8000}"
  echo "Expose HTTPS with Nginx on the VPS."
else
  docker compose --profile caddy --env-file .env up -d --build
fi

echo "Stack is running with Docker Model Runner model: ${MODEL_REF}"
echo "Health check: https://localhost/health"
