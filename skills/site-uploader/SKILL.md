---
name: site-uploader
description: Upload user-provided HTML files to a configured site-uploader API, with first-use credential setup, token validation, listing, search, and optional page passwords.
---

# Site Uploader

Use this skill when the user asks to publish, upload, list, search, or update a standalone HTML page through a site-uploader deployment.

The site homepage is informational only and never provides a public page list. For list or search questions, use the authenticated APIs below and return concise results; paginate when `pages` is greater than 1.

## First-use setup

1. Resolve the secrets file as `${CODEX_HOME:-$HOME/.codex}/secrets.env`.
2. If it does not exist, stop before making an upload request and ask the user to create it with:

   ```dotenv
   SITE_UPLOADER_TOKEN=replace-with-the-site-token
   SITE_UPLOADER_BASE_URL=https://your-site.example
   ```

   Do not invent, echo, commit, or place the token in chat. The user must provide the value privately in that file.
3. If the file exists, source it in a shell without printing it. Require both `SITE_UPLOADER_TOKEN` and `SITE_UPLOADER_BASE_URL`; never assume a deployment URL.
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

Only add `-F "access_password=1234"` when the user explicitly requests a page access password. It must be exactly four digits; omit it by default. Do not put the access password in the final response unless the user supplied or requests it.

Never include the token in command output, logs, generated files, or the final response. Return the API's `url` and `id` to the user. Do not upload a file merely to test credentials; use `/api/auth/check`.

If the configured URL is unavailable, report the failure. Do not invent a private-network fallback or deployment address; use one only when the user or local secrets explicitly provides it.

When the user provides an existing page ID and a replacement HTML file, use `PUT /api/sites/{id}` with the file instead of creating a second page. Confirm the replacement response and, when practical, compare the downloaded byte count with the local file. Never assume a successful upload truncated or transformed the file.

For the API contract and failure handling, read [references/api.md](references/api.md).
