# Cloudflare BFF Proxy (Production)

This worker provides same-origin proxy endpoints for the frontend:

- `/api/*` -> `https://vbsapi.cyblisswisdom.org/*`
- `/whep/*` -> `https://vbsrtc.cyblisswisdom.org/*`

## Security and behavior

- Keeps user `Authorization: Bearer ...` header intact
- Adds `Cf-Access-Client-Id/Secret` for upstream M2M access
- Uses `redirect: "manual"` and converts upstream `30x` auth challenge to `401` JSON
- Handles `OPTIONS` preflight and appends CORS headers
- Restricts browser origin via `ALLOWED_ORIGIN`

## Files

- `worker.js` - Worker logic
- `wrangler.toml` - Worker config and vars

## Required secrets

Run in this folder:

```bash
wrangler secret put CF_ACCESS_CLIENT_ID
wrangler secret put CF_ACCESS_CLIENT_SECRET
```

Use the **BFF** service token pair.

## Deploy

```bash
wrangler deploy
```

## Routes

Bind routes in Cloudflare Worker:

- `vbs.cyblisswisdom.org/api/*`
- `vbs.cyblisswisdom.org/whep/*`

## Origin allow list

`wrangler.toml` currently uses:

```toml
ALLOWED_ORIGIN = "https://vbs.cyblisswisdom.org"
```

If frontend domain changes, update this value and redeploy.

## Verify

```bash
curl -i -X POST "https://vbs.cyblisswisdom.org/api/v1/auth/admin/email-login" \
  -H "Content-Type: application/json" \
  --data '{"email":"vbs.engine.tg@gmail.com"}'
```

Expected: no more browser-side Access redirect/CORS loops.
