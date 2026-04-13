#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODEL_REF="${1:-hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF:Q6_K}"

cd "${ROOT_DIR}"

if [[ ! -f .env ]]; then
  cp .env.example .env
fi

./scripts/docker-model-run.sh "${MODEL_REF}"

docker compose --env-file .env up -d --build

echo "Stack is running with Docker Model Runner model: ${MODEL_REF}"
echo "Health check: https://localhost/health"
