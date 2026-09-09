# site-uploader API

- `GET /api/auth/check`: authenticated, side-effect-free token check. Returns `204 No Content` for a valid Bearer token, `401 Unauthorized` otherwise.
- `GET /api/sites`: public JSON list of uploaded sites.
- `POST /api/sites`: authenticated multipart upload. Required parts are `title` and `file`; `summary` is optional. The service limits the parsed multipart form to 20 MiB and the stored file stream to 10 MiB.
- `GET /s/{id}`: serves the uploaded HTML file.

The service is deployed on `100.99.0.5:18080`; Caddy on `100.99.0.6` proxies `site.tsio.top` to that Tailnet address. Upload responses contain an HTTPS URL based on the configured domain.
