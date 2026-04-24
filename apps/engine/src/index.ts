import { createServer, IncomingMessage, ServerResponse } from "node:http";
import { randomUUID } from "node:crypto";
import { spawn, ChildProcessByStdio } from "node:child_process";
import { Readable } from "node:stream";
import { URL } from "node:url";
import { createRemoteJWKSet, importSPKI, jwtVerify, type KeyLike, type JWTPayload } from "jose";
import WebSocket from "ws";

type OutputKey = "pgm" | "aux1" | "aux2" | "aux3" | "aux4";

interface RuntimeState {
  program: string;
  preview: string;
  aux: Record<"1" | "2" | "3" | "4", string>;
}

interface ProcessState {
  key: OutputKey;
  source: string;
  uri: string;
  proc: ChildProcessByStdio<null, Readable, Readable>;
}

interface RuntimeConfig {
  inputs: number;
  pgm_count: number;
  aux_count: number;
  input_sources?: string[];
  aux_sources?: Record<string, string>;
}

const controlHost = env("VBS_ENGINE_CONTROL_BIND_HOST", "0.0.0.0");
const controlPort = intEnv("VBS_ENGINE_CONTROL_BIND_PORT", 5010);

const defaultInputs: Record<string, string> = {};
for (let i = 1; i <= 8; i += 1) {
  const uri = env(`VBS_ENGINE_SRT_INPUT_${i}_URI`, "");
  if (uri) defaultInputs[`input${i}`] = uri;
}
let activeInputs: Record<string, string> = { ...defaultInputs };

const relayHost = env("VBS_ROUTE_PGM_RELAY_HOST", "");
const relayPort = intEnv("VBS_ROUTE_PGM_RELAY_PORT", 20030);
const relayPublicHost = env("VBS_ROUTE_PGM_PUBLIC_HOST", relayHost || "route.example.com");
const passphrase = requiredEnv("VBS_SRT_PASSPHRASE");
const latency = intEnv("VBS_ENGINE_PGM_SRT_LATENCY_MS", 200);

const consoleBase = env("VBS_CONSOLE_BASE_URL", "");
const telemetryEnabled = env("VBS_ENGINE_TELEMETRY_ENABLED", "1") !== "0" && consoleBase !== "";
const telemetryPath = env("VBS_ENGINE_TELEMETRY_WS_PATH", "/vbs/telemetry/ws");
const telemetryIntervalSec = Number(env("VBS_METRICS_INTERVAL_SEC", "1")) || 1;
const nodeId = env("VBS_NODE_ID", "vbs-engine");
const cfAccessJWT = env("VBS_CF_ACCESS_JWT", "");
const cfAccessClientID = env("VBS_CF_ACCESS_CLIENT_ID", "");
const cfAccessClientSecret = env("VBS_CF_ACCESS_CLIENT_SECRET", "");
const cfAccessAud = requiredEnv("VBS_CF_ACCESS_AUD");
const cfAccessTeamDomain = env("VBS_CF_ACCESS_TEAM_DOMAIN", "");
const cfAccessJWKSURL = env("VBS_CF_ACCESS_JWKS_URL", "");
const adminEmails = splitCSVLower(env("VBS_ADMIN_EMAILS", ""));
const nodeCNPrefix = env("VBS_NODE_CN_PREFIX", "vbs-node-").toLowerCase();
const consoleJWTIssuer = env("VBS_CONSOLE_JWT_ISSUER", "vbs-console");
const consoleJWTPublicKeys = splitCSVRaw(env("VBS_CONSOLE_JWT_PUBLIC_KEYS", ""));

const cfIssuer = resolveIssuer(cfAccessTeamDomain);
const resolvedJWKSURL = resolveJWKSURL(cfIssuer, cfAccessJWKSURL);
const jwksCacheSec = intEnv("VBS_CF_JWKS_CACHE_TTL_SEC", 3600);
const remoteJWKSet = createRemoteJWKSet(new URL(resolvedJWKSURL), {
  cacheMaxAge: Math.max(60, jwksCacheSec) * 1000,
});
const consolePublicKeyLoaders = consoleJWTPublicKeys.map((k) => parseEd25519PublicKey(k));

const state: RuntimeState = {
  program: "",
  preview: "",
  aux: {
    "1": "",
    "2": "",
    "3": "",
    "4": "",
  },
};
let runtimeConfig: RuntimeConfig = {
  inputs: Math.max(1, Object.keys(defaultInputs).length),
  pgm_count: 1,
  aux_count: 4,
};

const processes = new Map<OutputKey, ProcessState>();

const outputConfig: Record<OutputKey, { streamEnv: string; streamIdPubEnv: string; streamIdReadEnv: string; uuidEnv: string }> = {
  pgm: {
    streamEnv: "VBS_ENGINE_PGM_SRT_URI",
    streamIdPubEnv: "VBS_ENGINE_PGM_STREAMID_PUBLISH",
    streamIdReadEnv: "VBS_ENGINE_PGM_STREAMID_READ",
    uuidEnv: "VBS_PGM_STREAM_UUID",
  },
  aux1: {
    streamEnv: "VBS_ENGINE_AUX1_SRT_URI",
    streamIdPubEnv: "VBS_ENGINE_AUX1_STREAMID_PUBLISH",
    streamIdReadEnv: "VBS_ENGINE_AUX1_STREAMID_READ",
    uuidEnv: "VBS_AUX1_STREAM_UUID",
  },
  aux2: {
    streamEnv: "VBS_ENGINE_AUX2_SRT_URI",
    streamIdPubEnv: "VBS_ENGINE_AUX2_STREAMID_PUBLISH",
    streamIdReadEnv: "VBS_ENGINE_AUX2_STREAMID_READ",
    uuidEnv: "VBS_AUX2_STREAM_UUID",
  },
  aux3: {
    streamEnv: "VBS_ENGINE_AUX3_SRT_URI",
    streamIdPubEnv: "VBS_ENGINE_AUX3_STREAMID_PUBLISH",
    streamIdReadEnv: "VBS_ENGINE_AUX3_STREAMID_READ",
    uuidEnv: "VBS_AUX3_STREAM_UUID",
  },
  aux4: {
    streamEnv: "VBS_ENGINE_AUX4_SRT_URI",
    streamIdPubEnv: "VBS_ENGINE_AUX4_STREAMID_PUBLISH",
    streamIdReadEnv: "VBS_ENGINE_AUX4_STREAMID_READ",
    uuidEnv: "VBS_AUX4_STREAM_UUID",
  },
};

function env(name: string, defaultValue: string): string {
  return (process.env[name] ?? defaultValue).trim();
}

function requiredEnv(name: string): string {
  const value = env(name, "");
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}

function intEnv(name: string, defaultValue: number): number {
  const raw = env(name, "");
  if (!raw) return defaultValue;
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 ? n : defaultValue;
}

async function authorized(req: IncomingMessage): Promise<boolean> {
  const cfAssertion = String(req.headers["cf-access-jwt-assertion"] ?? "").trim();
  const auth = String(req.headers.authorization ?? "").trim();
  const raw = cfAssertion || (auth.toLowerCase().startsWith("bearer ") ? auth.slice(7).trim() : "");
  if (!raw) return false;
  const role = await resolveRoleFromToken(raw);
  return role === "admin" || role === "operator";
}

async function resolveRoleFromToken(raw: string): Promise<string> {
  try {
    const payload = decodePayload(raw);
    const iss = String(payload.iss ?? "").trim();
    if (iss && iss === consoleJWTIssuer) {
      return await resolveConsoleRole(raw);
    }
    return await resolveCloudflareRole(raw);
  } catch {
    return "";
  }
}

async function resolveCloudflareRole(raw: string): Promise<string> {
  const options: { audience: string; issuer?: string } = { audience: cfAccessAud };
  if (cfIssuer) options.issuer = cfIssuer;
  const { payload } = await jwtVerify(raw, remoteJWKSet, options);
  const role = String(payload.role ?? "").trim().toLowerCase();
  if (role) return role;
  const email = String(payload.email ?? "").trim().toLowerCase();
  const commonName = String(payload.common_name ?? "").trim().toLowerCase();
  if (adminEmails.includes(email)) return "admin";
  if (commonName.startsWith(nodeCNPrefix)) return "node";
  // Cloudflare Service Token 常見以 "<client_id>.access" 形式出現在 common_name。
  if (commonName.endsWith(".access")) return "node";
  return "";
}

async function resolveConsoleRole(raw: string): Promise<string> {
  for (const loader of consolePublicKeyLoaders) {
    try {
      const key = await loader;
      const { payload } = await jwtVerify(raw, key, {
        issuer: consoleJWTIssuer,
        audience: cfAccessAud,
      });
      const role = String(payload.role ?? "").trim().toLowerCase();
      if (role === "operator") {
        const subject = String(payload.sub ?? "").trim();
        const guestId = subject.startsWith("guest:") ? subject.slice("guest:".length) : "";
        const sessionVersion = Number(payload.sv ?? 0);
        const active = await introspectGuestSession(guestId, sessionVersion);
        if (!active) return "";
      }
      return role;
    } catch {
      // try next key
    }
  }
  return "";
}

async function introspectGuestSession(guestId: string, sessionVersion: number): Promise<boolean> {
  if (!guestId || !consoleBase) return false;
  try {
    const base = consoleBase.replace(/\/+$/, "");
    const authHeaders = accessHeaders();
    const res = await fetch(`${base}/api/v1/guest/introspect`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...authHeaders,
      },
      body: JSON.stringify({
        guest_id: guestId,
        session_version: sessionVersion,
      }),
    });
    if (!res.ok) return false;
    const out = (await res.json()) as { active?: boolean };
    return out.active === true;
  } catch {
    return false;
  }
}

function decodePayload(raw: string): JWTPayload {
  const parts = raw.split(".");
  if (parts.length < 2) return {};
  const json = Buffer.from(parts[1], "base64url").toString("utf8");
  return JSON.parse(json) as JWTPayload;
}

function splitCSVLower(raw: string): string[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim().toLowerCase())
    .filter((s) => s.length > 0);
}

function splitCSVRaw(raw: string): string[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

function normalizePEM(raw: string): string {
  return raw.replace(/\\n/g, "\n").trim();
}

async function parseEd25519PublicKey(raw: string): Promise<KeyLike> {
  const pem = normalizePEM(raw);
  return importSPKI(pem, "EdDSA");
}

function inputUri(source: string): string {
  if (activeInputs[source]) return activeInputs[source];
  if (source.startsWith("srt://")) return source;
  throw new Error(`unsupported source: ${source}`);
}

function outputUri(key: OutputKey): string {
  const explicit = env(outputConfig[key].streamEnv, "");
  if (explicit) return explicit;
  if (!relayHost) {
    throw new Error(`Missing ${outputConfig[key].streamEnv} and VBS_ROUTE_PGM_RELAY_HOST`);
  }
  const streamUUID = env(outputConfig[key].uuidEnv, randomUUID());
  const publish = env(outputConfig[key].streamIdPubEnv, `publish/${streamUUID}`);
  const read = env(outputConfig[key].streamIdReadEnv, `read/${streamUUID}`);
  const uri = `srt://${relayHost}:${relayPort}?mode=caller&transtype=live&streamid=${publish}&passphrase=${passphrase}&pbkeylen=32&latency=${latency}`;
  const readURL = `srt://${relayPublicHost}:${relayPort}?streamid=${read}&passphrase=${passphrase}&latency=${latency}`;
  console.log(`[engine][output] ${key} publish=${publish} read_url=${readURL}`);
  return uri;
}

function startOutput(key: OutputKey, source: string): void {
  const current = processes.get(key);
  const nextSourceUri = inputUri(source);
  const nextOutputUri = outputUri(key);

  if (current && current.source === source && current.uri === nextOutputUri) {
    return;
  }
  if (current) {
    current.proc.kill("SIGTERM");
    processes.delete(key);
  }

  const args = [
    "-hide_banner",
    "-loglevel",
    "warning",
    "-re",
    "-i",
    nextSourceUri,
    "-c",
    "copy",
    "-f",
    "mpegts",
    nextOutputUri,
  ];
  const proc = spawn("ffmpeg", args, { stdio: ["ignore", "pipe", "pipe"] });
  proc.stdout.on("data", (d) => process.stdout.write(`[engine][${key}] ${d}`));
  proc.stderr.on("data", (d) => process.stderr.write(`[engine][${key}] ${d}`));
  proc.on("exit", (code) => {
    console.error(`[engine][${key}] ffmpeg exited code=${code}`);
    processes.delete(key);
  });
  processes.set(key, { key, source, uri: nextOutputUri, proc });
}

function stopOutput(key: OutputKey): void {
  const current = processes.get(key);
  if (!current) return;
  current.proc.kill("SIGTERM");
  processes.delete(key);
}

function applyRouting(): void {
  if (!state.program) {
    stopOutput("pgm");
    stopOutput("aux1");
    stopOutput("aux2");
    stopOutput("aux3");
    stopOutput("aux4");
    return;
  }
  startOutput("pgm", state.program);
  if (runtimeConfig.aux_count >= 1) startOutput("aux1", state.aux["1"]); else stopOutput("aux1");
  if (runtimeConfig.aux_count >= 2) startOutput("aux2", state.aux["2"]); else stopOutput("aux2");
  if (runtimeConfig.aux_count >= 3) startOutput("aux3", state.aux["3"]); else stopOutput("aux3");
  if (runtimeConfig.aux_count >= 4) startOutput("aux4", state.aux["4"]); else stopOutput("aux4");
}

function pickInput(n: number): string {
  const key = `input${n}`;
  if (activeInputs[key]) return key;
  return Object.keys(activeInputs).sort()[0] ?? "";
}

function bootstrapStateFromInputs(): void {
  state.program = pickInput(1);
  state.preview = pickInput(2) || state.program;
  state.aux["1"] = pickInput(1) || state.program;
  state.aux["2"] = pickInput(2) || state.program;
  state.aux["3"] = pickInput(3) || state.program;
  state.aux["4"] = pickInput(4) || state.program;
}

function ensureSourceAllowed(source: string, cfg: RuntimeConfig): boolean {
  if (source.startsWith("srt://")) return true;
  if (!source.startsWith("input")) return false;
  const n = Number(source.slice("input".length));
  return Number.isFinite(n) && n >= 1 && n <= cfg.inputs && Boolean(activeInputs[`input${n}`]);
}

function applyRuntimeConfig(next: RuntimeConfig): RuntimeConfig {
  if (!Number.isInteger(next.inputs) || next.inputs < 1 || next.inputs > 8) {
    throw new Error("inputs must be integer between 1 and 8");
  }
  if (!Number.isInteger(next.pgm_count) || next.pgm_count !== 1) {
    throw new Error("pgm_count currently supports only 1");
  }
  if (!Number.isInteger(next.aux_count) || next.aux_count < 0 || next.aux_count > 4) {
    throw new Error("aux_count must be integer between 0 and 4");
  }
  if (next.input_sources && next.input_sources.length > 8) {
    throw new Error("input_sources cannot exceed 8 entries");
  }

  const updatedInputs: Record<string, string> = {};
  const sourceList = next.input_sources && next.input_sources.length > 0
    ? next.input_sources
    : Array.from({ length: next.inputs }, (_, idx) => activeInputs[`input${idx + 1}`] || defaultInputs[`input${idx + 1}`] || "");

  for (let i = 0; i < next.inputs; i += 1) {
      const src = String(sourceList[i] ?? "").trim();
      if (!src) throw new Error(`input_sources[${i}] is empty`);
      updatedInputs[`input${i + 1}`] = src;
  }
  activeInputs = updatedInputs;

  if (next.aux_sources) {
    for (const [ch, srcRaw] of Object.entries(next.aux_sources)) {
      if (!["1", "2", "3", "4"].includes(ch)) throw new Error("aux_sources keys must be 1..4");
      const src = String(srcRaw ?? "").trim();
      if (!src) throw new Error(`aux_sources[${ch}] is empty`);
      if (!ensureSourceAllowed(src, next)) throw new Error(`aux_sources[${ch}] source out of range`);
      state.aux[ch as "1" | "2" | "3" | "4"] = src;
    }
  }

  if (!ensureSourceAllowed(state.program, next)) state.program = pickInput(1);
  if (!ensureSourceAllowed(state.preview, next)) state.preview = pickInput(2) || state.program;
  for (const ch of ["1", "2", "3", "4"] as const) {
    if (!ensureSourceAllowed(state.aux[ch], next)) state.aux[ch] = pickInput(Number(ch)) || state.program;
  }

  runtimeConfig = {
    inputs: next.inputs,
    pgm_count: next.pgm_count,
    aux_count: next.aux_count,
    input_sources: next.input_sources?.map((s) => String(s)),
    aux_sources: next.aux_sources ? { ...next.aux_sources } : undefined,
  };
  applyRouting();
  return runtimeConfig;
}

async function readBody(req: IncomingMessage): Promise<any> {
  const chunks: Buffer[] = [];
  for await (const chunk of req) chunks.push(Buffer.from(chunk));
  if (!chunks.length) return {};
  return JSON.parse(Buffer.concat(chunks).toString("utf-8"));
}

function writeJson(res: ServerResponse, code: number, body: unknown): void {
  const raw = Buffer.from(JSON.stringify(body));
  res.statusCode = code;
  res.setHeader("Content-Type", "application/json");
  res.setHeader("Content-Length", String(raw.length));
  res.end(raw);
}

function telemetryWsUrl(base: string, path: string): string {
  const u = new URL(base);
  u.protocol = u.protocol === "https:" ? "wss:" : "ws:";
  u.pathname = path.startsWith("/") ? path : `/${path}`;
  u.search = "";
  u.hash = "";
  return u.toString();
}

async function sendTelemetryLoop(): Promise<void> {
  const authHeaders = accessHeaders();
  const wsUrl = telemetryWsUrl(consoleBase, telemetryPath);
  const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));
  while (true) {
    try {
      const payload = {
        node_id: nodeId,
        node_type: "engine",
        ts_ms: Date.now(),
        metrics: { workers: processes.size, cpu_pct: 0 },
        auth_mode: "cf_jwt",
      };
      const raw = JSON.stringify(payload);
      if (Buffer.byteLength(raw, "utf8") <= 255) {
        await new Promise<void>((resolve, reject) => {
          const ws = new WebSocket(wsUrl, {
            headers: authHeaders,
          });
          ws.on("open", () => ws.send(raw));
          ws.on("message", () => undefined);
          ws.on("error", reject);
          ws.on("close", () => resolve());
        });
      }
    } catch (err) {
      console.error(`[engine][telemetry] ${String(err)}`);
    }
    await wait(Math.max(200, telemetryIntervalSec * 1000));
  }
}

function accessHeaders(): Record<string, string> {
  if (cfAccessJWT) {
    return {
      Authorization: `Bearer ${cfAccessJWT}`,
    };
  }
  if (cfAccessClientID && cfAccessClientSecret) {
    return {
      "Cf-Access-Client-Id": cfAccessClientID,
      "Cf-Access-Client-Secret": cfAccessClientSecret,
    };
  }
  throw new Error("Missing Cloudflare Access credentials: set VBS_CF_ACCESS_JWT or VBS_CF_ACCESS_CLIENT_ID/VBS_CF_ACCESS_CLIENT_SECRET");
}

const server = createServer(async (req, res) => {
  try {
    if (req.method === "GET" && req.url === "/healthz") {
      writeJson(res, 200, { status: "ok", engine: "eyevinn-ts" });
      return;
    }
    if (req.method === "GET" && req.url === "/api/v1/switch/state") {
      if (!(await authorized(req))) return writeJson(res, 401, { error: "unauthorized" });
      writeJson(res, 200, state);
      return;
    }
    if (req.method === "GET" && req.url === "/api/v1/runtime/config") {
      if (!(await authorized(req))) return writeJson(res, 401, { error: "unauthorized" });
      writeJson(res, 200, { config: runtimeConfig, state });
      return;
    }
    if (req.method === "POST" && req.url === "/api/v1/runtime/config/apply") {
      if (!(await authorized(req))) return writeJson(res, 401, { error: "unauthorized" });
      const body = (await readBody(req)) as RuntimeConfig;
      const applied = applyRuntimeConfig({
        inputs: Number(body.inputs),
        pgm_count: Number(body.pgm_count),
        aux_count: Number(body.aux_count),
        input_sources: body.input_sources,
        aux_sources: body.aux_sources,
      });
      writeJson(res, 200, { applied: true, config: applied, state });
      return;
    }
    if (req.method === "POST" && req.url === "/api/v1/switch/program") {
      if (!(await authorized(req))) return writeJson(res, 401, { error: "unauthorized" });
      const body = await readBody(req);
      state.program = String(body.source ?? "").trim();
      if (!state.program) return writeJson(res, 400, { error: "source required" });
      applyRouting();
      writeJson(res, 200, { applied: true, program: state.program });
      return;
    }
    if (req.method === "POST" && req.url === "/api/v1/switch/preview") {
      if (!(await authorized(req))) return writeJson(res, 401, { error: "unauthorized" });
      const body = await readBody(req);
      state.preview = String(body.source ?? "").trim();
      if (!state.preview) return writeJson(res, 400, { error: "source required" });
      writeJson(res, 200, { applied: true, preview: state.preview });
      return;
    }
    if (req.method === "POST" && req.url === "/api/v1/switch/aux") {
      if (!(await authorized(req))) return writeJson(res, 401, { error: "unauthorized" });
      const body = await readBody(req);
      const channel = String(body.channel ?? "");
      const source = String(body.source ?? "").trim();
      if (!["1", "2", "3", "4"].includes(channel)) return writeJson(res, 400, { error: "channel must be 1..4" });
      if (!source) return writeJson(res, 400, { error: "source required" });
      state.aux[channel as "1" | "2" | "3" | "4"] = source;
      applyRouting();
      writeJson(res, 200, { applied: true, channel, source });
      return;
    }
    writeJson(res, 404, { error: "not found" });
  } catch (err) {
    writeJson(res, 500, { error: String(err) });
  }
});

process.on("SIGTERM", () => {
  for (const item of processes.values()) item.proc.kill("SIGTERM");
  process.exit(0);
});
process.on("SIGINT", () => {
  for (const item of processes.values()) item.proc.kill("SIGTERM");
  process.exit(0);
});

bootstrapStateFromInputs();
if (Object.keys(activeInputs).length > 0) {
  applyRouting();
} else {
  runtimeConfig.aux_count = 0;
  console.log("[engine] no input URIs configured at boot; waiting runtime config apply");
}
server.listen(controlPort, controlHost, () => {
  console.log(`[engine] Eyevinn TS control API on ${controlHost}:${controlPort}`);
});

if (telemetryEnabled) {
  sendTelemetryLoop().catch((err) => console.error(`[engine][telemetry] fatal ${String(err)}`));
} else {
  console.log("[engine][telemetry] disabled");
}

function resolveIssuer(teamDomain: string): string {
  const cleaned = teamDomain.replace(/^https?:\/\//, "").replace(/\/+$/, "").trim();
  return cleaned ? `https://${cleaned}` : "";
}

function resolveJWKSURL(issuer: string, explicit: string): string {
  const raw = explicit.trim();
  if (raw) return raw;
  if (!issuer) throw new Error("Missing VBS_CF_ACCESS_TEAM_DOMAIN or VBS_CF_ACCESS_JWKS_URL");
  return `${issuer}/cdn-cgi/access/certs`;
}
