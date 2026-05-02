# Cloudflare BFF Proxy (Production)

This worker provides same-origin proxy endpoints for the frontend:

- `/api/*` -> `https://vbsapi.cyblisswisdom.org/*`
- `/whep/*` -> `https://vbsrtc.cyblisswisdom.org/*`
- `/engine/*` -> `https://vbsengine.cyblisswisdom.org/*` (strips `/engine` prefix)
- `/route/*` -> `https://vbsroute.cyblisswisdom.org/*` (strips `/route` prefix)

## Security and behavior

- Keeps user `X-VBS-Authorization` header intact
- `/api/*`: injects service token only when `X-VBS-Authorization` is absent (pre-auth endpoints)
- For `/engine/*`, `/route/*`, `/whep/*`: forwards caller service token if provided; injects upstream token only when missing
- Uses `redirect: "manual"` and converts upstream `30x` auth challenge to `401` JSON
- Handles `OPTIONS` preflight and appends CORS headers
- Restricts browser origin via `ALLOWED_ORIGIN`

## Files

- `worker.js` - Worker logic
- `wrangler.toml` - Worker config and vars

## Required secrets

Run in this folder (set upstream-specific secrets if needed):

```bash
wrangler secret put CF_ACCESS_CLIENT_ID
wrangler secret put CF_ACCESS_CLIENT_SECRET
wrangler secret put ENGINE_CF_ACCESS_CLIENT_ID
wrangler secret put ENGINE_CF_ACCESS_CLIENT_SECRET
wrangler secret put ROUTE_CF_ACCESS_CLIENT_ID
wrangler secret put ROUTE_CF_ACCESS_CLIENT_SECRET
```

Use the **BFF** service token for generic `/api`/`/whep`; set dedicated upstream tokens for `/engine` and `/route` when Access policies require them.

## Deploy

```bash
wrangler deploy
```

## Routes

Bind routes in Cloudflare Worker:

- `vbs.cyblisswisdom.org/api/*`
- `vbs.cyblisswisdom.org/whep/*`
- `vbsapi.cyblisswisdom.org/engine*`
- `vbsapi.cyblisswisdom.org/route*`

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
