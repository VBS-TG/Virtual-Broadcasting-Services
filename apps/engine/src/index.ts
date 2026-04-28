import { timingSafeEqual } from "node:crypto";
import { createServer, IncomingMessage, ServerResponse } from "node:http";
import { URL } from "node:url";
import { createRemoteJWKSet, jwtVerify } from "jose";
import WebSocket from "ws";
import { normalizeShowConfig, validateShowConfig, type ShowConfig } from "./showConfig.js";

interface RuntimeState {
  program: string;
  preview: string;
  aux: Record<"1" | "2" | "3" | "4", string>;
}

interface RuntimeConfig {
  inputs: number;
  pgm_count: number;
  aux_count: number;
  input_sources?: string[];
  aux_sources?: Record<string, string>;
}

interface OpenLiveSource {
  id: string;
  name?: string;
  streamType?: string;
  stream_type?: string;
  address?: string;
}

interface OpenLiveProduction {
  id: string;
  name?: string;
}

type OpenLiveListResponse<T> = T[] | { items?: T[]; data?: T[]; results?: T[] };

const controlHost = env("VBS_ENGINE_CONTROL_BIND_HOST", "0.0.0.0");
const controlPort = intEnv("VBS_ENGINE_CONTROL_BIND_PORT", 5000);
const openLiveBaseURL = requiredEnv("VBS_EYEVINN_OPENLIVE_BASE_URL");
const openLiveApplyPath = env("VBS_EYEVINN_OPENLIVE_APPLY_PATH", "/api/v1/runtime/config/apply");
const openLiveStatePath = env("VBS_EYEVINN_OPENLIVE_STATE_PATH", "/api/v1/switch/state");
const openLiveHealthPath = env("VBS_EYEVINN_OPENLIVE_HEALTH_PATH", "/healthz");
const openLiveAuthToken = env("VBS_EYEVINN_OPENLIVE_AUTH_TOKEN", "");
const openLiveReadyPath = env("VBS_EYEVINN_OPENLIVE_READY_PATH", "/ready");
const openLiveProductionID = env("VBS_EYEVINN_OPENLIVE_PRODUCTION_ID", "");
const openLiveProductionName = env("VBS_EYEVINN_OPENLIVE_PRODUCTION_NAME", "vbs-main");
const openLiveProgramInput = env("VBS_EYEVINN_OPENLIVE_PROGRAM_INPUT", "program");
const openLivePreviewInput = env("VBS_EYEVINN_OPENLIVE_PREVIEW_INPUT", "preview");
const openLiveAuxInputPrefix = env("VBS_EYEVINN_OPENLIVE_AUX_INPUT_PREFIX", "aux");
const openLiveAutoActivate = env("VBS_EYEVINN_OPENLIVE_AUTO_ACTIVATE", "1") !== "0";

const consoleBase = env("VBS_CONSOLE_BASE_URL", "");
const telemetryEnabled = env("VBS_ENGINE_TELEMETRY_ENABLED", "1") !== "0" && consoleBase !== "";
const telemetryPath = env("VBS_ENGINE_TELEMETRY_WS_PATH", "/vbs/telemetry/ws");
const telemetryIntervalSec = Number(env("VBS_METRICS_INTERVAL_SEC", "1")) || 1;
const nodeId = env("VBS_NODE_ID", "vbs-engine");
const cfAccessClientID = env("VBS_CF_ACCESS_CLIENT_ID", "");
const cfAccessClientSecret = env("VBS_CF_ACCESS_CLIENT_SECRET", "");
const cfAccessAud = requiredEnv("VBS_CF_ACCESS_AUD");
const cfAccessTeamDomain = env("VBS_CF_ACCESS_TEAM_DOMAIN", "");
const cfAccessJWKSURL = env("VBS_CF_ACCESS_JWKS_URL", "");
const nodeCNPrefix = env("VBS_NODE_CN_PREFIX", "vbs-node-").toLowerCase();

const cfIssuer = resolveIssuer(cfAccessTeamDomain);
const resolvedJWKSURL = resolveJWKSURL(cfIssuer, cfAccessJWKSURL);
const jwksCacheSec = intEnv("VBS_CF_JWKS_CACHE_TTL_SEC", 3600);
const remoteJWKSet = createRemoteJWKSet(new URL(resolvedJWKSURL), {
  cacheMaxAge: Math.max(60, jwksCacheSec) * 1000,
});

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
  inputs: 8,
  pgm_count: 1,
  aux_count: 4,
};
let activeProductionID = "";

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

function joinURL(base: string, path: string): string {
  const b = base.replace(/\/+$/, "");
  const p = path.startsWith("/") ? path : `/${path}`;
  return `${b}${p}`;
}

function openLiveHeaders(json: boolean): Record<string, string> {
  const headers: Record<string, string> = {};
  if (json) headers["Content-Type"] = "application/json";
  if (openLiveAuthToken) headers.Authorization = `Bearer ${openLiveAuthToken}`;
  return headers;
}

async function openLiveRequest(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = {
    ...openLiveHeaders(false),
    ...(init.headers as Record<string, string> | undefined),
  };
  return fetch(joinURL(openLiveBaseURL, path), { ...init, headers });
}

async function openLiveJSON<T>(path: string): Promise<T> {
  const res = await openLiveRequest(path, { method: "GET" });
  if (!res.ok) throw new Error(`open live get ${path} status=${res.status}`);
  return (await res.json()) as T;
}

function asArray<T>(payload: OpenLiveListResponse<T>): T[] {
  if (Array.isArray(payload)) return payload;
  if (Array.isArray(payload.items)) return payload.items;
  if (Array.isArray(payload.data)) return payload.data;
  if (Array.isArray(payload.results)) return payload.results;
  return [];
}

function pickString(obj: Record<string, unknown>, keys: string[]): string {
  for (const k of keys) {
    const v = obj[k];
    if (typeof v === "string" && v.trim()) return v.trim();
  }
  return "";
}

async function openLiveSend(path: string, method: "POST" | "PATCH" | "DELETE", body?: unknown): Promise<Response> {
  return openLiveRequest(path, {
    method,
    headers: openLiveHeaders(true),
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

function inputKey(index: number): string {
  return `input${index}`;
}

function sourceIDForInput(index: number): string {
  return `vbs-${inputKey(index)}`;
}

function sourceNameForInput(index: number): string {
  return `VBS ${inputKey(index)}`;
}

function mixerInputForAux(channel: string): string {
  return `${openLiveAuxInputPrefix}${channel}`;
}

async function ensureProduction(): Promise<string> {
  if (activeProductionID) return activeProductionID;
  if (openLiveProductionID) {
    activeProductionID = openLiveProductionID;
    return activeProductionID;
  }
  const listRaw = await openLiveJSON<OpenLiveListResponse<OpenLiveProduction>>("/api/v1/productions");
  const list = asArray(listRaw);
  const found = list.find((p) => String(p.name ?? "") === openLiveProductionName);
  if (found?.id) {
    activeProductionID = found.id;
    return activeProductionID;
  }
  const create = await openLiveSend("/api/v1/productions", "POST", { name: openLiveProductionName });
  if (!create.ok) {
    const raw = await create.text();
    throw new Error(`open live create production status=${create.status} body=${raw.slice(0, 200)}`);
  }
  // Some Open Live builds do not return the production id directly on create.
  const created = (await create.json()) as Record<string, unknown>;
  const candidateID = pickString(created, ["id", "_id", "productionId", "production_id", "uuid"]);
  if (candidateID) {
    activeProductionID = candidateID;
    return activeProductionID;
  }
  // Fallback: re-list and pick by name after creation.
  const listAfterCreateRaw = await openLiveJSON<OpenLiveListResponse<OpenLiveProduction>>("/api/v1/productions");
  const listAfterCreate = asArray(listAfterCreateRaw);
  const createdByName = listAfterCreate.find((p) => String(p.name ?? "") === openLiveProductionName);
  activeProductionID = String(createdByName?.id ?? "").trim();
  if (!activeProductionID) throw new Error("open live create production missing id");
  return activeProductionID;
}

async function listOpenLiveSources(): Promise<OpenLiveSource[]> {
  const raw = await openLiveJSON<OpenLiveListResponse<OpenLiveSource>>("/api/v1/sources");
  return asArray(raw);
}

async function ensureSource(index: number, address: string): Promise<string> {
  const id = sourceIDForInput(index);
  const all = await listOpenLiveSources();
  const found = all.find((s) => String(s.id) === id);
  const payload = {
    id,
    name: sourceNameForInput(index),
    streamType: "srt",
    address,
  };
  if (!found) {
    const create = await openLiveSend("/api/v1/sources", "POST", payload);
    if (!create.ok) {
      const raw = await create.text();
      throw new Error(`open live create source status=${create.status} body=${raw.slice(0, 200)}`);
    }
    return id;
  }
  const currentAddress = String(found.address ?? "");
  const currentType = String(found.streamType ?? found.stream_type ?? "").toLowerCase();
  if (currentAddress === address && (currentType === "srt" || currentType === "")) return id;
  const patch = await openLiveSend(`/api/v1/sources/${encodeURIComponent(id)}`, "PATCH", {
    address,
    streamType: "srt",
  });
  if (!patch.ok) {
    const raw = await patch.text();
    throw new Error(`open live patch source status=${patch.status} body=${raw.slice(0, 200)}`);
  }
  return id;
}

async function assignProductionInput(productionID: string, mixerInput: string, sourceID: string): Promise<void> {
  const res = await openLiveSend(`/api/v1/productions/${encodeURIComponent(productionID)}/sources`, "POST", {
    mixerInput,
    sourceId: sourceID,
  });
  if (res.ok) return;
  const raw = await res.text();
  // Production can be stale/raced; resolve once and retry.
  if (res.status === 404 && raw.toLowerCase().includes("production not found")) {
    activeProductionID = "";
    const freshID = await ensureProduction();
    const retry = await openLiveSend(`/api/v1/productions/${encodeURIComponent(freshID)}/sources`, "POST", {
      mixerInput,
      sourceId: sourceID,
    });
    if (retry.ok) return;
    const retryRaw = await retry.text();
    throw new Error(`open live assign source retry status=${retry.status} body=${retryRaw.slice(0, 200)}`);
  }
  throw new Error(`open live assign source status=${res.status} body=${raw.slice(0, 200)}`);
}

function sourceIDFromSelection(source: string): string {
  if (source.startsWith("input")) {
    const n = Number(source.slice("input".length));
    if (Number.isFinite(n) && n >= 1 && n <= 8) return sourceIDForInput(n);
  }
  throw new Error(`unsupported source selection: ${source}`);
}

async function activateProduction(productionID: string): Promise<void> {
  if (!openLiveAutoActivate) return;
  const res = await openLiveSend(`/api/v1/productions/${encodeURIComponent(productionID)}/activate`, "POST");
  // 200/409 都視為可用（有些流程若已啟動會返回衝突）
  if (res.ok || res.status === 409) return;
  const raw = await res.text();
  throw new Error(`open live activate production status=${res.status} body=${raw.slice(0, 200)}`);
}

/** 與 Route 相同：Console Orchestrator 送 Cf-Access-Client-Id/Secret 時自動通過（須與本節點 VBS_CF_ACCESS_* 一致）。 */
function matchInboundAccessServiceToken(req: IncomingMessage): boolean {
  const wantId = cfAccessClientID.trim();
  const wantSecret = cfAccessClientSecret.trim();
  if (!wantId || !wantSecret) return false;
  const gotId = String(req.headers["cf-access-client-id"] ?? "").trim();
  const gotSecret = String(req.headers["cf-access-client-secret"] ?? "").trim();
  if (!gotId || !gotSecret) return false;
  if (gotId.length !== wantId.length || gotSecret.length !== wantSecret.length) return false;
  try {
    const bi = Buffer.from(gotId);
    const wi = Buffer.from(wantId);
    const bs = Buffer.from(gotSecret);
    const ws = Buffer.from(wantSecret);
    return timingSafeEqual(bi, wi) && timingSafeEqual(bs, ws);
  } catch {
    return false;
  }
}

async function authorized(req: IncomingMessage): Promise<boolean> {
  // Release policy:
  // 1) preferred M2M service token from Console orchestrator
  // 2) fallback Cloudflare JWT assertion, but only node identity may pass
  if (matchInboundAccessServiceToken(req)) return true;
  const cfAssertion = String(req.headers["cf-access-jwt-assertion"] ?? "").trim();
  if (!cfAssertion) return false;
  const role = await resolveCloudflareRole(cfAssertion);
  return role === "node";
}

async function resolveCloudflareRole(raw: string): Promise<string> {
  const options: { audience: string; issuer?: string } = { audience: cfAccessAud };
  if (cfIssuer) options.issuer = cfIssuer;
  const { payload } = await jwtVerify(raw, remoteJWKSet, options);
  const role = String(payload.role ?? "").trim().toLowerCase();
  if (role) return role;
  const commonName = String(payload.common_name ?? "").trim().toLowerCase();
  if (commonName.startsWith(nodeCNPrefix)) return "node";
  // Cloudflare Service Token 常見以 "<client_id>.access" 形式出現在 common_name。
  if (commonName.endsWith(".access")) return "node";
  return "";
}

function ensureSourceAllowed(source: string, cfg: RuntimeConfig): boolean {
  if (!source) return false;
  if (source.startsWith("srt://")) return true;
  if (!source.startsWith("input")) return false;
  const n = Number(source.slice("input".length));
  return Number.isFinite(n) && n >= 1 && n <= cfg.inputs;
}

async function fetchOpenLiveState(): Promise<RuntimeState> {
  const res = await openLiveRequest(openLiveStatePath, { method: "GET" });
  if (!res.ok) throw new Error(`open live state status=${res.status}`);
  return (await res.json()) as RuntimeState;
}

async function applyRuntimeConfig(next: RuntimeConfig): Promise<RuntimeConfig> {
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
  const sourceList = next.input_sources ?? [];
  for (let i = 0; i < sourceList.length; i += 1) {
    const src = String(sourceList[i] ?? "").trim();
    if (!src) throw new Error(`input_sources[${i}] is empty`);
    if (!src.startsWith("srt://")) throw new Error(`input_sources[${i}] must be srt:// URI`);
  }

  if (next.aux_sources) {
    for (const [ch, srcRaw] of Object.entries(next.aux_sources)) {
      if (!["1", "2", "3", "4"].includes(ch)) throw new Error("aux_sources keys must be 1..4");
      const src = String(srcRaw ?? "").trim();
      if (!src) throw new Error(`aux_sources[${ch}] is empty`);
      if (!ensureSourceAllowed(src, next)) throw new Error(`aux_sources[${ch}] source out of range`);
    }
  }

  const productionID = await ensureProduction();
  for (let i = 0; i < sourceList.length; i += 1) {
    await ensureSource(i + 1, sourceList[i]);
  }
  await activateProduction(productionID);
  // Apply selected buses to Open Live production mapping.
  await assignProductionInput(productionID, openLiveProgramInput, sourceIDFromSelection(state.program || "input1"));
  await assignProductionInput(productionID, openLivePreviewInput, sourceIDFromSelection(state.preview || "input2"));
  for (const ch of ["1", "2", "3", "4"] as const) {
    if (Number(ch) > next.aux_count) continue;
    const selected = String(next.aux_sources?.[ch] ?? state.aux[ch] ?? `input${ch}`);
    await assignProductionInput(productionID, mixerInputForAux(ch), sourceIDFromSelection(selected));
  }
  // Optional compatibility path for non-default Open Live apply endpoint.
  if (openLiveApplyPath && openLiveApplyPath !== "/api/v1/runtime/config/apply") {
    const compat = await openLiveSend(openLiveApplyPath, "POST", next);
    if (!compat.ok) {
      const raw = await compat.text();
      throw new Error(`open live compat apply status=${compat.status} body=${raw.slice(0, 200)}`);
    }
  }

  runtimeConfig = {
    inputs: next.inputs,
    pgm_count: next.pgm_count,
    aux_count: next.aux_count,
    input_sources: next.input_sources?.map((s) => String(s)),
    aux_sources: next.aux_sources ? { ...next.aux_sources } : undefined,
  };
  try {
    const remoteState = await fetchOpenLiveState();
    state.program = String(remoteState.program ?? "");
    state.preview = String(remoteState.preview ?? "");
    state.aux = {
      "1": String(remoteState.aux?.["1"] ?? ""),
      "2": String(remoteState.aux?.["2"] ?? ""),
      "3": String(remoteState.aux?.["3"] ?? ""),
      "4": String(remoteState.aux?.["4"] ?? ""),
    };
  } catch {
    // Open Live state endpoint may not exist on all deployments.
  }
  return runtimeConfig;
}

async function readBody(req: IncomingMessage): Promise<any> {
  const chunks: Buffer[] = [];
  for await (const chunk of req) chunks.push(Buffer.from(chunk));
  if (!chunks.length) return {};
  return JSON.parse(Buffer.concat(chunks).toString("utf-8"));
}

async function readBodyLimited(req: IncomingMessage, maxBytes: number): Promise<Buffer> {
  const chunks: Buffer[] = [];
  let total = 0;
  for await (const chunk of req) {
    const b = Buffer.from(chunk);
    total += b.length;
    if (total > maxBytes) throw new Error("body too large");
    chunks.push(b);
  }
  return Buffer.concat(chunks);
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
        metrics: { workers: 0, cpu_pct: 0 },
        auth_mode: "cf_service_token",
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
  if (cfAccessClientID && cfAccessClientSecret) {
    return {
      "Cf-Access-Client-Id": cfAccessClientID,
      "Cf-Access-Client-Secret": cfAccessClientSecret,
    };
  }
  throw new Error("Missing Cloudflare Access credentials: set VBS_CF_ACCESS_CLIENT_ID and VBS_CF_ACCESS_CLIENT_SECRET");
}

const server = createServer(async (req, res) => {
  try {
    if (req.method === "GET" && req.url === "/healthz") {
      let openLiveOK = false;
      try {
        const r = await openLiveRequest(openLiveHealthPath, { method: "GET" });
        if (r.ok) {
          openLiveOK = true;
        } else {
          const ready = await openLiveRequest(openLiveReadyPath, { method: "GET" });
          openLiveOK = ready.ok;
        }
      } catch {
        openLiveOK = false;
      }
      writeJson(res, openLiveOK ? 200 : 503, { status: openLiveOK ? "ok" : "degraded", engine: "eyevinn-openlive-adapter", open_live_ok: openLiveOK });
      return;
    }
    if (req.method === "GET" && req.url === "/api/v1/switch/state") {
      if (!(await authorized(req))) return writeJson(res, 401, { error: "unauthorized" });
      try {
        const remoteState = await fetchOpenLiveState();
        state.program = String(remoteState.program ?? "");
        state.preview = String(remoteState.preview ?? "");
        state.aux = {
          "1": String(remoteState.aux?.["1"] ?? ""),
          "2": String(remoteState.aux?.["2"] ?? ""),
          "3": String(remoteState.aux?.["3"] ?? ""),
          "4": String(remoteState.aux?.["4"] ?? ""),
        };
      } catch {
        // fallback to last known state
      }
      writeJson(res, 200, state);
      return;
    }
    if (req.method === "GET" && req.url === "/api/v1/runtime/config") {
      if (!(await authorized(req))) return writeJson(res, 401, { error: "unauthorized" });
      writeJson(res, 200, { config: runtimeConfig, state });
      return;
    }
    if ((req.method === "POST" && req.url === "/api/v1/runtime/config/apply") || (req.method === "PUT" && req.url === "/api/v1/runtime/config")) {
      if (!(await authorized(req))) return writeJson(res, 401, { error: "unauthorized" });
      const body = (await readBody(req)) as RuntimeConfig;
      const applied = await applyRuntimeConfig({
        inputs: Number(body.inputs),
        pgm_count: Number(body.pgm_count),
        aux_count: Number(body.aux_count),
        input_sources: body.input_sources,
        aux_sources: body.aux_sources,
      });
      writeJson(res, 200, { applied: true, config: applied, state });
      return;
    }
    if (req.method === "POST" && req.url === "/api/v1/show-config/apply") {
      if (!(await authorized(req))) return writeJson(res, 401, { error: "unauthorized" });
      try {
        const buf = await readBodyLimited(req, 512 * 1024);
        const parsed = JSON.parse(buf.toString("utf-8")) as ShowConfig;
        normalizeShowConfig(parsed);
        const verr = validateShowConfig(parsed, runtimeConfig.inputs);
        if (verr) return writeJson(res, 400, { error: verr });
        console.log(
          `[engine][show-config] applied panel=${parsed.switcher.panel_id} sources=${parsed.sources?.length ?? 0} cells=${parsed.multiview.cells?.length ?? 0}`,
        );
        writeJson(res, 200, {
          applied: true,
          node: "engine",
          inputs: runtimeConfig.inputs,
        });
      } catch (e) {
        writeJson(res, 400, { error: String(e) });
      }
      return;
    }
    if (req.method === "POST" && req.url === "/api/v1/switch/program") {
      if (!(await authorized(req))) return writeJson(res, 401, { error: "unauthorized" });
      const body = await readBody(req);
      const source = String(body.source ?? "").trim();
      if (!source) return writeJson(res, 400, { error: "source required" });
      const productionID = await ensureProduction();
      await assignProductionInput(productionID, openLiveProgramInput, sourceIDFromSelection(source));
      state.program = source;
      writeJson(res, 200, { applied: true, program: state.program });
      return;
    }
    if (req.method === "POST" && req.url === "/api/v1/switch/preview") {
      if (!(await authorized(req))) return writeJson(res, 401, { error: "unauthorized" });
      const body = await readBody(req);
      const source = String(body.source ?? "").trim();
      if (!source) return writeJson(res, 400, { error: "source required" });
      const productionID = await ensureProduction();
      await assignProductionInput(productionID, openLivePreviewInput, sourceIDFromSelection(source));
      state.preview = source;
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
      const productionID = await ensureProduction();
      await assignProductionInput(productionID, mixerInputForAux(channel), sourceIDFromSelection(source));
      state.aux[channel as "1" | "2" | "3" | "4"] = source;
      writeJson(res, 200, { applied: true, channel, source });
      return;
    }
    writeJson(res, 404, { error: "not found" });
  } catch (err) {
    writeJson(res, 500, { error: String(err) });
  }
});

process.on("SIGTERM", () => process.exit(0));
process.on("SIGINT", () => process.exit(0));

server.listen(controlPort, controlHost, () => {
  console.log(`[engine] Eyevinn Open Live adapter API on ${controlHost}:${controlPort}`);
  console.log(`[engine] Open Live base: ${openLiveBaseURL}`);
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
