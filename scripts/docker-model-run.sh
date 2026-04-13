#!/usr/bin/env bash
set -euo pipefail

MODEL_REF="${1:-hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF:Q6_K}"
HF_TOKEN_FILE="${HF_TOKEN_FILE:-${HOME}/.cache/huggingface/token}"
LOW_MEM_MODEL_REF="${LOW_MEM_MODEL_REF:-hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF:Q4_K}"
AUTO_FALLBACK_LOW_MEM="${AUTO_FALLBACK_LOW_MEM:-1}"
UNLOAD_EXISTING_MODELS="${UNLOAD_EXISTING_MODELS:-1}"
ENV_FILE="${ENV_FILE:-.env}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker CLI not found in PATH" >&2
  exit 1
fi

if ! docker model version >/dev/null 2>&1; then
  echo "Docker Model Runner is not available or not enabled." >&2
  echo "Enable Model Runner first in Docker Desktop, or install docker-model-plugin on Docker Engine." >&2
  exit 1
fi

if [[ -z "${HF_TOKEN:-}" ]] && [[ -f "${HF_TOKEN_FILE}" ]]; then
  export HF_TOKEN
  HF_TOKEN="$(tr -d '\r\n' < "${HF_TOKEN_FILE}")"
fi

if [[ -z "${HF_TOKEN:-}" ]]; then
  echo "HF_TOKEN is not set and token file was not found at ${HF_TOKEN_FILE}." >&2
  echo "Use one of these methods before running this script:" >&2
  echo "  1) export HF_TOKEN=\$(cat ~/.cache/huggingface/token)" >&2
  echo "  2) env PATH=\"${HOME}/.local/bin:\$PATH\" hf auth login" >&2
  exit 1
fi

run_model() {
  local model_ref="$1"

  echo "Starting model: ${model_ref}"
  docker model run --detach "${model_ref}"
}

if [[ "${UNLOAD_EXISTING_MODELS}" == "1" ]]; then
  echo "Unloading existing running models to free memory..."
  docker model unload --all || true
fi

ACTIVE_MODEL_REF="${MODEL_REF}"
if ! run_model "${MODEL_REF}"; then
  echo "Primary model failed to initialize: ${MODEL_REF}" >&2
  echo "Tip: a larger context (for example 16384) increases KV cache memory and can trigger OOM." >&2
  echo "Note: LLM_CTX_SIZE in .env only trims chat history in backend requests; it does not reduce runner startup memory." >&2

  if [[ "${AUTO_FALLBACK_LOW_MEM}" == "1" ]] && [[ "${MODEL_REF}" != "${LOW_MEM_MODEL_REF}" ]]; then
    echo "Retrying with low-memory fallback model: ${LOW_MEM_MODEL_REF}" >&2
    run_model "${LOW_MEM_MODEL_REF}"
    ACTIVE_MODEL_REF="${LOW_MEM_MODEL_REF}"
  else
    echo "Fallback disabled or identical model requested. Exiting." >&2
    exit 1
  fi
fi

if [[ -f "${ENV_FILE}" ]]; then
  if grep -q '^LLM_MODEL_NAME=' "${ENV_FILE}"; then
    sed -i "s|^LLM_MODEL_NAME=.*|LLM_MODEL_NAME=${ACTIVE_MODEL_REF}|" "${ENV_FILE}"
  else
    printf '\nLLM_MODEL_NAME=%s\n' "${ACTIVE_MODEL_REF}" >> "${ENV_FILE}"
  fi
fi

echo "Model is warming up in Docker Model Runner."
echo "Checking running models:"
docker model ps || true

echo "Default .env values for backend are already aligned with DMR:"
echo "LLM_BASE_URL=http://model-runner.docker.internal/engines"
echo "LLM_MODEL_NAME=${ACTIVE_MODEL_REF}"
