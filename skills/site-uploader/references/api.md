# site-uploader API

- `GET /api/auth/check`: authenticated, side-effect-free token check. Returns `204 No Content` for a valid Bearer token, `401 Unauthorized` otherwise.
- `GET /api/sites`: authenticated paginated JSON list. Query `page` (default 1) and `page_size` (default 20, max 100). Response is `{items, page, page_size, total, pages}`.
- `GET /api/sites/search?q=...`: authenticated fuzzy search over title, summary, and ID, with the same pagination contract. The local tokenizer splits on whitespace/punctuation and individual CJK characters.
- `POST /api/sites`: authenticated multipart upload. Required parts are `title` and `file`; `summary` and `access_password` are optional. `access_password`, when present, must be exactly four ASCII digits. The service limits the parsed multipart form to 20 MiB and the stored file stream to 10 MiB.
- `GET /s/{id}`: serves the uploaded HTML file. Password-protected pages show a password form; submit the four-digit password with `POST /s/{id}` and the service sets an HttpOnly cookie.

The service address is deployment-specific and must come from `SITE_UPLOADER_BASE_URL`. Upload responses contain a URL based on the service's configured domain.
