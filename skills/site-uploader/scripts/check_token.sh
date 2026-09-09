#!/usr/bin/env bash
set -euo pipefail
codex_root="${CODEX_HOME:-${HOME}/.codex}"
secrets_file="${codex_root}/secrets.env"
if [[ ! -f "${secrets_file}" ]]; then echo "MISSING_SECRETS:${secrets_file}"; exit 2; fi
# shellcheck disable=SC1090
source "${secrets_file}"
: "${SITE_UPLOADER_TOKEN:?SITE_UPLOADER_TOKEN is missing from ${secrets_file}}"
base_url="${SITE_UPLOADER_BASE_URL:-https://site.tsio.top}"
base_url="${base_url%/}"
status="$(curl -fsS -o /dev/null -w '%{http_code}' -H "Authorization: Bearer ${SITE_UPLOADER_TOKEN}" "${base_url}/api/auth/check" || true)"
case "${status}" in
  204) echo "TOKEN_OK:${base_url}";;
  401) echo "TOKEN_INVALID:${base_url}"; exit 3;;
  '') echo "SERVICE_UNAVAILABLE:${base_url}"; exit 4;;
  *) echo "TOKEN_CHECK_HTTP_${status}:${base_url}"; exit 4;;
esac
