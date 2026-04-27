/** Show Config（與 `pkg/showconfig`、`packages/shared/schemas/show-config.v1.schema.json` 對齊）。 */

export const DEFAULT_SCHEMA_VERSION = "1.0";

export interface ShowConfig {
  schema_version: string;
  profile: ProductionProfile;
  sources: SourceSlot[];
  switcher: SwitcherMapping;
  multiview: MultiviewLayout;
}

export interface ProductionProfile {
  mode: string;
  target?: ResolutionTarget;
  hop_overrides?: Record<string, Record<string, unknown>>;
}

export interface ResolutionTarget {
  width: number;
  height: number;
  frame_rate: number;
}

export interface SourceSlot {
  slot_id: string;
  display_name: string;
  short_label?: string;
  group_id?: string;
}

export interface SwitcherMapping {
  panel_id: string;
  rows: SwitcherRow[];
}

export interface SwitcherRow {
  buttons: SwitcherButton[];
}

export interface SwitcherButton {
  position: number;
  bind: SwitcherBind;
}

export interface SwitcherBind {
  kind: string;
  ref?: string;
}

export interface MultiviewLayout {
  template_id: string;
  cells: MultiviewCell[];
}

export interface MultiviewCell {
  cell_index: number;
  role: string;
  source_ref: string;
}

export function normalizeShowConfig(cfg: ShowConfig): void {
  if (!cfg.schema_version) cfg.schema_version = DEFAULT_SCHEMA_VERSION;
}

function parseInputSlot(slot: string, maxInputs: number): string | null {
  const s = slot.trim().toLowerCase();
  if (!s.startsWith("input")) return "必須為 inputN 格式";
  const numStr = s.slice("input".length);
  const n = Number(numStr);
  if (!Number.isFinite(n) || n < 1 || n > maxInputs) return `必須為 input1..input${maxInputs}`;
  return null;
}

/** 驗證失敗時回傳錯誤字串；成功回傳 null。 */
export function validateShowConfig(cfg: ShowConfig, inputs: number): string | null {
  if (!cfg.schema_version) return "請先 Normalize 或提供 schema_version";
  if (cfg.schema_version !== DEFAULT_SCHEMA_VERSION) return `不支援的 schema_version（目前僅支援 ${DEFAULT_SCHEMA_VERSION}）`;
  if (!Number.isInteger(inputs) || inputs < 1 || inputs > 8) return "inputs 必須介於 1~8（用於交叉驗證）";

  const mode = cfg.profile.mode.trim().toLowerCase();
  if (mode !== "pipeline_locked" && mode !== "hop_override") return "profile.mode 必須為 pipeline_locked 或 hop_override";

  if (cfg.profile.target) {
    const t = cfg.profile.target;
    if (t.width <= 0 || t.height <= 0) return "profile.target 之 width/height 必須為正整數";
    if (t.frame_rate <= 0 || t.frame_rate > 240) return "profile.target.frame_rate 必須為合理正數（建議 ≤240）";
  }

  const seen = new Set<string>();
  for (let i = 0; i < cfg.sources.length; i++) {
    const src = cfg.sources[i];
    const slot = src.slot_id.trim().toLowerCase();
    const pe = parseInputSlot(slot, inputs);
    if (pe) return `sources[${i}].slot_id: ${pe}`;
    if (!src.display_name.trim()) return `sources[${i}].display_name 不可為空`;
    if (seen.has(slot)) return `重複的 slot_id: ${slot}`;
    seen.add(slot);
  }

  if (!cfg.switcher.panel_id.trim()) return "switcher.panel_id 不可為空";

  for (let ri = 0; ri < cfg.switcher.rows.length; ri++) {
    const row = cfg.switcher.rows[ri];
    for (let bi = 0; bi < row.buttons.length; bi++) {
      const btn = row.buttons[bi];
      const kind = btn.bind.kind.trim().toLowerCase();
      if (!["input", "black", "bars"].includes(kind))
        return `switcher.rows[${ri}].buttons[${bi}]: bind.kind 必須為 input|black|bars`;
      if (kind === "input") {
        const ref = String(btn.bind.ref ?? "").trim().toLowerCase();
        const re = parseInputSlot(ref, inputs);
        if (re) return `switcher.rows[${ri}].buttons[${bi}].bind.ref: ${re}`;
      }
    }
  }

  if (!cfg.multiview.template_id.trim()) return "multiview.template_id 不可為空";

  for (let ci = 0; ci < cfg.multiview.cells.length; ci++) {
    const cell = cfg.multiview.cells[ci];
    const role = cell.role.trim().toLowerCase();
    if (!["preview_large", "program_large", "thumb", "off"].includes(role))
      return `multiview.cells[${ci}].role 必須為 preview_large|program_large|thumb|off`;
    const ref = cell.source_ref.trim().toLowerCase();
    if (ref && ref !== "off" && !ref.startsWith("input")) return `multiview.cells[${ci}].source_ref 必須為空、off 或 inputN`;
    if (ref.startsWith("input")) {
      const re = parseInputSlot(ref, inputs);
      if (re) return `multiview.cells[${ci}].source_ref: ${re}`;
    }
  }

  return null;
}
