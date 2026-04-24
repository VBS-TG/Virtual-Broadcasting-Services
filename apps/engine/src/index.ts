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

const controlHost = env("VBS_ENGINE_CONTROL_BIND_HOST", "0.0.0.0");
const controlPort = intEnv("VBS_ENGINE_CONTROL_BIND_PORT", 5010);

const input1 = requiredEnv("VBS_ENGINE_SRT_INPUT_1_URI");
const input2 = requiredEnv("VBS_ENGINE_SRT_INPUT_2_URI");

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
  program: "input1",
  preview: "input2",
  aux: {
    "1": "input1",
    "2": "input2",
    "3": "input1",
    "4": "input2",
  },
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
  const auth = String(req.headers.authorization ?? "");
  if (!auth.toLowerCase().startsWith("bearer ")) return false;
  const raw = auth.slice(7).trim();
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
  if (!guestId || !consoleBase || !cfAccessJWT) return false;
  try {
    const base = consoleBase.replace(/\/+$/, "");
    const res = await fetch(`${base}/api/v1/guest/introspect`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${cfAccessJWT}`,
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
  if (source === "input1") return input1;
  if (source === "input2") return input2;
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

function applyRouting(): void {
  startOutput("pgm", state.program);
  startOutput("aux1", state.aux["1"]);
  startOutput("aux2", state.aux["2"]);
  startOutput("aux3", state.aux["3"]);
  startOutput("aux4", state.aux["4"]);
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
  if (!cfAccessJWT) throw new Error("Missing VBS_CF_ACCESS_JWT");
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
            headers: {
              Authorization: `Bearer ${cfAccessJWT}`,
            },
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

applyRouting();
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
