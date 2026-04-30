# Cloudflare BFF Proxy (Permanent Fix)

This worker provides same-origin proxy endpoints for the frontend:

- `/api/*` -> `https://vbsapi.cyblisswisdom.org/*`
- `/whep/*` -> `https://vbsrtc.cyblisswisdom.org/*`

It injects Cloudflare Access service-token headers server-side, so browser XHR no longer gets redirected to Access interactive pages.

## 1) Prerequisites

- Install Wrangler and login:

```bash
npm i -g wrangler
wrangler login
```

## 2) Configure secrets

Run in this folder (`infra/cloudflare-bff`):

```bash
wrangler secret put CF_ACCESS_CLIENT_ID
wrangler secret put CF_ACCESS_CLIENT_SECRET
```

Use the BFF service token pair (recommended dedicated token for this worker).

## 3) Deploy worker

```bash
wrangler deploy
```

## 4) Bind routes

Add these routes in Worker settings (or in `wrangler.toml`):

- `vbs.cyblisswisdom.org/api/*`
- `vbs.cyblisswisdom.org/whep/*`

## 5) Important frontend note

- Keep frontend API calls as same-origin (`/api/...`)
- Do not set `VITE_API_BASE_URL` in production
- `_redirects` should **not** forward `/api/*` to external origin

## 6) Verify

```bash
curl -i -X POST "https://vbs.cyblisswisdom.org/api/v1/auth/admin/email-login" \
  -H "Content-Type: application/json" \
  --data '{"email":"vbs.engine.tg@gmail.com"}'
```

Expected: no more 302 to `/cdn-cgi/access/authorized`, no CORS redirect errors in browser.
