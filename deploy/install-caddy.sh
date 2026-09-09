#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID} -ne 0 ]]; then
  echo 'Run with sudo: sudo bash install-caddy.sh [PREPARED_DIRECTORY]' >&2
  exit 1
fi

# When invoked from the prepared directory, no argument is needed. An explicit
# directory is still accepted for callers storing the script elsewhere.
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
prepared_dir="$(realpath "${1:-${script_dir}}")"
python3 - "${prepared_dir}" <<'PY'
import hashlib
import json
from pathlib import Path
import sys
import urllib.request

directory = Path(sys.argv[1])
metadata = json.loads((directory / 'metadata.json').read_text())
assert hashlib.sha256(Path('/etc/caddy/Caddyfile').read_bytes()).hexdigest() == metadata['before_sha256'], 'Caddyfile changed since preparation; regenerate the candidate.'
assert hashlib.sha256((directory / 'Caddyfile').read_bytes()).hexdigest() == metadata['candidate_sha256'], 'Candidate changed since validation.'
with urllib.request.urlopen('http://127.0.0.1:2019/config/', timeout=5) as response:
    current = json.load(response)
assert current == json.loads((directory / 'runtime.before.json').read_text()), 'Runtime configuration changed; regenerate the candidate.'
PY

caddy validate --config "${prepared_dir}/Caddyfile" --adapter caddyfile
backup_file="/etc/caddy/Caddyfile.bak.site-uploader.$(date -u +%Y%m%dT%H%M%SZ)"
cp -p /etc/caddy/Caddyfile "${backup_file}"
install -o root -g root -m 0644 "${prepared_dir}/Caddyfile" /etc/caddy/Caddyfile.site-uploader.new
mv /etc/caddy/Caddyfile.site-uploader.new /etc/caddy/Caddyfile
if ! systemctl reload caddy; then
  cp -p "${backup_file}" /etc/caddy/Caddyfile
  curl --fail --silent --show-error --max-time 10 -H 'Content-Type: application/json' \
    --data-binary "@${prepared_dir}/runtime.before.json" http://127.0.0.1:2019/load
  echo "Reload failed; restored disk and runtime configuration. Backup: ${backup_file}" >&2
  exit 1
fi
systemctl is-active caddy
python3 - "${prepared_dir}" <<'PY'
import json
from pathlib import Path
import sys
import urllib.request
with urllib.request.urlopen('http://127.0.0.1:2019/config/', timeout=5) as response:
    current = json.load(response)
assert current == json.loads((Path(sys.argv[1]) / 'runtime.proposed.json').read_text()), 'Loaded configuration does not match the prepared configuration.'
print('Caddy configuration persisted and loaded successfully.')
PY
printf 'Backup: %s\n' "${backup_file}"
