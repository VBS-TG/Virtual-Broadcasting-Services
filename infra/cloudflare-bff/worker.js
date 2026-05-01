/**
 * VBS same-origin BFF proxy for Cloudflare.
 *
 * Routes:
 * - /api/*  -> https://vbsapi.cyblisswisdom.org/*
 * - /whep/* -> https://vbsrtc.cyblisswisdom.org/*
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
      return proxyToUpstream(request, env, env.API_ORIGIN, origin);
    }

    if (path.startsWith("/whep/")) {
      return proxyToUpstream(request, env, env.RTC_ORIGIN, origin);
    }

    return withCORS(new Response("Not Found", { status: 404 }), env, origin);
  },
};

async function proxyToUpstream(request, env, upstreamOrigin, requestOrigin) {
  if (!upstreamOrigin) {
    return withCORS(jsonError(500, "missing upstream origin"), env, requestOrigin);
  }

  const incomingUrl = new URL(request.url);
  const upstreamUrl = new URL(incomingUrl.pathname + incomingUrl.search, upstreamOrigin);

  const headers = new Headers(request.headers);
  headers.delete("host");
  // Strict separation: human JWT uses X-VBS-Authorization only.
  // Do not forward generic Authorization to avoid auth-source mixing.
  headers.delete("authorization");
  headers.set("cf-access-client-id", env.CF_ACCESS_CLIENT_ID || "");
  headers.set("cf-access-client-secret", env.CF_ACCESS_CLIENT_SECRET || "");
  headers.set("x-forwarded-host", incomingUrl.host);
  headers.set("x-forwarded-proto", incomingUrl.protocol.replace(":", ""));

  const init = {
    method: request.method,
    headers,
    redirect: "manual",
  };

  if (request.method !== "GET" && request.method !== "HEAD") {
    init.body = request.body;
  }

  const upstreamResp = await fetch(upstreamUrl.toString(), init);

  // Access challenge or upstream redirect should not be forwarded as a browser redirect.
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

  // Reject cross-origin preflight that does not match policy.
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
