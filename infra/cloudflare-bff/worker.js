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

    if (path.startsWith("/api/")) {
      return proxyToUpstream(request, env, env.API_ORIGIN);
    }

    if (path.startsWith("/whep/")) {
      return proxyToUpstream(request, env, env.RTC_ORIGIN);
    }

    return new Response("Not Found", { status: 404 });
  },
};

async function proxyToUpstream(request, env, origin) {
  if (!origin) {
    return jsonError(500, "missing upstream origin");
  }

  const incomingUrl = new URL(request.url);
  const upstreamUrl = new URL(incomingUrl.pathname + incomingUrl.search, origin);

  const headers = new Headers(request.headers);
  headers.delete("host");
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
  return new Response(upstreamResp.body, {
    status: upstreamResp.status,
    statusText: upstreamResp.statusText,
    headers: upstreamResp.headers,
  });
}

function jsonError(status, message) {
  return new Response(JSON.stringify({ error: message }), {
    status,
    headers: { "content-type": "application/json" },
  });
}
