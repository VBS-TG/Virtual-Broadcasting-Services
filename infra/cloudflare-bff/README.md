# Cloudflare BFF Proxy (Production)

This Worker provides **same-origin** proxy paths for the frontend (`https://vbs.cyblisswisdom.org`) so the browser can call relative URLs (`/api/*`, `/whep/*`, …) without CORS issues.

## Upstream mapping

- `/api/*` → `API_ORIGIN` (e.g. `https://vbsapi.cyblisswisdom.org`)
- `/whep/*` → `RTC_ORIGIN` (e.g. `https://vbsrtc.cyblisswisdom.org`)
- `/engine/*` → `ENGINE_ORIGIN/*` (prefix `/engine` stripped)
- `/route/*` → `ROUTE_ORIGIN/*` (prefix `/route` stripped)

## Behavior (passthrough)

- **No** `Cf-Access-Client-Id` / `Cf-Access-Client-Secret` injection in the Worker.
- **No** conditional logic on `X-VBS-Authorization`; incoming headers are forwarded as received (except `Host`, which must be dropped so the upstream URL’s host is used).
- **`redirect: "manual"`** on upstream `fetch`; `3xx` responses are mapped to a JSON `401` so the browser does not follow redirects blindly.
- **CORS**: `OPTIONS` preflight and `Access-Control-*` on responses per `ALLOWED_ORIGIN`.

**Network vs application auth**

- **Cloudflare Access** (policies on each hostname / tunnel) is the **first layer** (“is this caller allowed to reach this origin?”).
- **`console_backend`** validates **`X-VBS-Authorization`** (Console-issued JWT) and related rules — **second layer**. The Worker does not interpret JWTs.

**Deploy note:** With no Worker-side service token, ensure Access policies (and/or tunnel config) allow browser traffic from `vbs.cyblisswisdom.org` to reach `API_ORIGIN` / `RTC_ORIGIN` as intended (including public login paths if applicable).

## Files

- `worker.js` — Worker logic
- `wrangler.toml` — Vars (`API_ORIGIN`, `ALLOWED_ORIGIN`, …)

Secrets for **injecting** Access tokens into the Worker are **no longer required** for this design.

## Deploy

```bash
wrangler deploy
```

## Example routes (Cloudflare dashboard)

Bind routes such as:

- `vbs.cyblisswisdom.org/api/*`
- `vbs.cyblisswisdom.org/whep/*`
- `vbsapi.cyblisswisdom.org/engine*`
- `vbsapi.cyblisswisdom.org/route*`

## Origin allow list

`wrangler.toml` sets `ALLOWED_ORIGIN` (e.g. `https://vbs.cyblisswisdom.org`). Update if the Pages hostname changes.

## Verify (login path must be allowed by Access)

```bash
curl -i -X POST "https://vbs.cyblisswisdom.org/api/v1/auth/admin/email-login" \
  -H "Content-Type: application/json" \
  --data '{"email":"your-admin@example.com"}'
```
