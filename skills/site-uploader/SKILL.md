---
name: site-uploader
description: Upload user-provided HTML files to site.tsio.top through the authenticated site-uploader API, with first-use credential setup and token validation.
---

# Site Uploader

Use this skill when the user asks to publish, upload, or update a standalone HTML page on `site.tsio.top`.

## First-use setup

1. Resolve the secrets file as `${CODEX_HOME:-$HOME/.codex}/secrets.env`.
2. If it does not exist, stop before making an upload request and ask the user to create it with:

   ```dotenv
   SITE_UPLOADER_TOKEN=replace-with-the-site-token
   SITE_UPLOADER_BASE_URL=https://site.tsio.top
   ```

   Do not invent, echo, commit, or place the token in chat. The user must provide the value privately in that file.
3. If the file exists, source it in a shell without printing it. Require `SITE_UPLOADER_TOKEN`; default `SITE_UPLOADER_BASE_URL` to `https://site.tsio.top`.
4. Validate the token before uploading with the read-only endpoint:

   ```sh
   curl -fsS -o /dev/null -w '%{http_code}' \
     -H "Authorization: Bearer ${SITE_UPLOADER_TOKEN}" \
     "${SITE_UPLOADER_BASE_URL}/api/auth/check"
   ```

   HTTP 204 means valid. HTTP 401 means the token is wrong or expired; ask the user to correct the secrets file. Treat network, TLS, 5xx, and any other status as an unavailable service and report it without retrying uploads.

## Upload

Ask for or infer a page title and optional summary. Resolve the requested HTML path to an absolute path, verify it is a regular file, then upload:

```sh
curl -fsS -X POST "${SITE_UPLOADER_BASE_URL}/api/sites" \
  -H "Authorization: Bearer ${SITE_UPLOADER_TOKEN}" \
  -F "title=${TITLE}" \
  -F "summary=${SUMMARY}" \
  -F "file=@${HTML_PATH}"
```

Never include the token in command output, logs, generated files, or the final response. Return the API's `url` and `id` to the user. Do not upload a file merely to test credentials; use `/api/auth/check`.

If the public HTTPS path is unavailable from the current network, retry the read-only token check and upload through the Tailnet fallback `http://100.99.0.5:18080`, preserving the same paths and headers. Only use that fallback when the host can reach the Tailnet address.

For the API contract and failure handling, read [references/api.md](references/api.md).
