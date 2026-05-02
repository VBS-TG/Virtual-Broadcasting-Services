/**
 * VBS same-origin BFF proxy for Cloudflare (pure forward).
 *
 * Routes:
 * - /api/*    -> API_ORIGIN/*
 * - /vbs/*    -> API_ORIGIN/* (telemetry ingest WSS, etc.; SPA hostname MUST route here — see README)
 * - /whep/*   -> RTC_ORIGIN/*
 * - /engine/* -> ENGINE_ORIGIN/* (strips /engine prefix)
 * - /route/*  -> ROUTE_ORIGIN/*  (strips /route prefix)
 *
 * No auth injection or header stripping beyond deleting Host (required for upstream fetch).
 * Identity is enforced at Cloudflare Access (network) and console_backend (application).
 *
 * WebSocket: requests with Upgrade: websocket are forwarded with fetch(); the runtime
 * handles 101 Switching Protocols and tunnels the socket to the upstream.
 *
 * Deploy: bind this Worker to vbs.cyblisswisdom.org/vbs/* (not only /api/*). If /vbs/*
 * is not routed here, the browser may receive HTTP 200 HTML from Pages (SPA), not 101.
 */
export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const path = url.pathname;
    const origin = request.headers.get("origin") || "";

    if (request.method === "OPTIONS") {
      return handlePreflight(request, env, origin);
    }

    if (path.startsWith("/api/")) {
      return proxyToUpstream(request, env, {
        upstreamOrigin: env.API_ORIGIN,
        requestOrigin: origin,
        stripPrefix: "",
      });
    }

    if (path.startsWith("/vbs/")) {
      return proxyToUpstream(request, env, {
        upstreamOrigin: env.API_ORIGIN,
        requestOrigin: origin,
        stripPrefix: "",
      });
    }

    if (path.startsWith("/whep/")) {
      return proxyToUpstream(request, env, {
        upstreamOrigin: env.RTC_ORIGIN,
        requestOrigin: origin,
        stripPrefix: "",
      });
    }

    if (path === "/engine" || path.startsWith("/engine/")) {
      return proxyToUpstream(request, env, {
        upstreamOrigin: env.ENGINE_ORIGIN,
        requestOrigin: origin,
        stripPrefix: "/engine",
      });
    }

    if (path === "/route" || path.startsWith("/route/")) {
      return proxyToUpstream(request, env, {
        upstreamOrigin: env.ROUTE_ORIGIN,
        requestOrigin: origin,
        stripPrefix: "/route",
      });
    }

    return withCORS(new Response("Not Found", { status: 404 }), env, origin);
  },
};

async function proxyToUpstream(request, env, opts) {
  const upstreamOrigin = String(opts?.upstreamOrigin || "").trim();
  const requestOrigin = String(opts?.requestOrigin || "");
  const stripPrefix = String(opts?.stripPrefix || "");
  if (!upstreamOrigin) {
    return withCORS(jsonError(500, "missing upstream origin"), env, requestOrigin);
  }

  const incomingUrl = new URL(request.url);
  let upstreamPath = incomingUrl.pathname;
  if (stripPrefix && upstreamPath.startsWith(stripPrefix)) {
    upstreamPath = upstreamPath.slice(stripPrefix.length);
    if (!upstreamPath.startsWith("/")) upstreamPath = "/" + upstreamPath;
  }
  const upstreamUrl = new URL(upstreamPath + incomingUrl.search, upstreamOrigin);

  const headers = new Headers(request.headers);
  headers.delete("host");

  if (String(request.headers.get("Upgrade") || "").toLowerCase() === "websocket") {
    return fetch(upstreamUrl.toString(), {
      headers,
      method: request.method,
      redirect: "manual",
    });
  }

  const init = {
    method: request.method,
    headers,
    redirect: "manual",
  };

  if (request.method !== "GET" && request.method !== "HEAD") {
    init.body = request.body;
  }

  const upstreamResp = await fetch(upstreamUrl.toString(), init);

  if (upstreamResp.status >= 300 && upstreamResp.status < 400) {
    return withCORS(
      jsonError(401, "unauthorized: upstream auth challenge"),
      env,
      requestOrigin
    );
  }

  const downstreamResp = new Response(upstreamResp.body, {
    status: upstreamResp.status,
    statusText: upstreamResp.statusText,
    headers: upstreamResp.headers,
  });
  return withCORS(downstreamResp, env, requestOrigin);
}

function jsonError(status, message) {
  return new Response(JSON.stringify({ error: message }), {
    status,
    headers: { "content-type": "application/json" },
  });
}

function handlePreflight(request, env, requestOrigin) {
  const reqMethod = request.headers.get("access-control-request-method") || "";
  const reqHeaders = request.headers.get("access-control-request-headers") || "";
  const allowedOrigin = resolveAllowedOrigin(env, requestOrigin);

  if (!allowedOrigin) {
    return new Response(null, { status: 403 });
  }

  const headers = new Headers();
  headers.set("access-control-allow-origin", allowedOrigin);
  headers.set("access-control-allow-methods", reqMethod || "GET,POST,PUT,PATCH,DELETE,OPTIONS");
  headers.set("access-control-allow-headers", reqHeaders || "x-vbs-authorization,content-type");
  headers.set("access-control-allow-credentials", "true");
  headers.set("access-control-max-age", "86400");
  headers.set("vary", "Origin");
  return new Response(null, { status: 204, headers });
}

function withCORS(response, env, requestOrigin) {
  const allowedOrigin = resolveAllowedOrigin(env, requestOrigin);
  if (!allowedOrigin) return response;

  const headers = new Headers(response.headers);
  headers.set("access-control-allow-origin", allowedOrigin);
  headers.set("access-control-allow-credentials", "true");
  headers.set("vary", "Origin");

  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  });
}

function resolveAllowedOrigin(env, requestOrigin) {
  const configured = String(env.ALLOWED_ORIGIN || "").trim();
  if (!configured) return "";
  if (!requestOrigin) return configured;
  return requestOrigin === configured ? configured : "";
}
