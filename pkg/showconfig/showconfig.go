// Package showconfig defines Show Config payload shape and validation（對齊 .cursorrules Show Config 正式規格）。
package showconfig

import (
	"fmt"
	"strconv"
	"strings"
)

const DefaultSchemaVersion = "1.0"

// ShowConfig 為 Console 權威之製作規格文件（與 Runtime SRT 綁定分離，需與 input 路數交叉驗證）。
type ShowConfig struct {
	SchemaVersion string             `json:"schema_version"`
	Profile       ProductionProfile  `json:"profile"`
	Sources       []SourceSlot       `json:"sources"`
	Switcher      SwitcherMapping    `json:"switcher"`
	Multiview     MultiviewLayout    `json:"multiview"`
}

// ProductionProfile 描述端到端畫質政策（細節由各節點 adapter 轉譯）。
type ProductionProfile struct {
	Mode         string                    `json:"mode"` // pipeline_locked | hop_override
	Target       *ResolutionTarget         `json:"target,omitempty"`
	HopOverrides map[string]map[string]any `json:"hop_overrides,omitempty"`
}

// ResolutionTarget 全域目標解析度與幀率。
type ResolutionTarget struct {
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	FrameRate float64 `json:"frame_rate"`
}

// SourceSlot 對齊 input1..inputN 之顯示名稱與標籤。
type SourceSlot struct {
	SlotID      string `json:"slot_id"`
	DisplayName string `json:"display_name"`
	ShortLabel  string `json:"short_label,omitempty"`
	GroupID     string `json:"group_id,omitempty"`
}

// SwitcherMapping 導播面板槽位綁定。
type SwitcherMapping struct {
	PanelID string        `json:"panel_id"`
	Rows    []SwitcherRow `json:"rows"`
}

// SwitcherRow 單列按鈕。
type SwitcherRow struct {
	Buttons []SwitcherButton `json:"buttons"`
}

// SwitcherButton 單鍵。
type SwitcherButton struct {
	Position int          `json:"position"`
	Bind     SwitcherBind `json:"bind"`
}

// SwitcherBind 綁定目標。
type SwitcherBind struct {
	Kind string `json:"kind"` // input | black | bars
	Ref  string `json:"ref,omitempty"`
}

// MultiviewLayout Multiview 格子與來源對應。
type MultiviewLayout struct {
	TemplateID string           `json:"template_id"`
	Cells      []MultiviewCell  `json:"cells"`
}

// MultiviewCell 單格。
type MultiviewCell struct {
	CellIndex int    `json:"cell_index"`
	Role      string `json:"role"`
	SourceRef string `json:"source_ref"`
}

// Default 回傳可序列化之空預設（供首次 GET）。
func Default() ShowConfig {
	return ShowConfig{
		SchemaVersion: DefaultSchemaVersion,
		Profile: ProductionProfile{
			Mode: "pipeline_locked",
			Target: &ResolutionTarget{
				Width: 1920, Height: 1080, FrameRate: 60,
			},
		},
		Sources:   []SourceSlot{},
		Switcher:  SwitcherMapping{PanelID: "main_atem_like", Rows: []SwitcherRow{}},
		Multiview: MultiviewLayout{TemplateID: "grid_4x4", Cells: []MultiviewCell{}},
	}
}

// Normalize 填寫預設 schema 版本（指標傳入以便 PUT 後儲存一致）。
func Normalize(cfg *ShowConfig) {
	if cfg == nil {
		return
	}
	if cfg.SchemaVersion == "" {
		cfg.SchemaVersion = DefaultSchemaVersion
	}
}

// Validate 檢查結構與與目前 Engine 啟用路數 inputs（1~8）一致。
func Validate(cfg ShowConfig, inputs int) error {
	if cfg.SchemaVersion == "" {
		return fmt.Errorf("請先 Normalize 或提供 schema_version")
	}
	if cfg.SchemaVersion != DefaultSchemaVersion {
		return fmt.Errorf("不支援的 schema_version（目前僅支援 %s）", DefaultSchemaVersion)
	}
	if inputs < 1 || inputs > 8 {
		return fmt.Errorf("inputs 必須介於 1~8（用於交叉驗證）")
	}
	mode := strings.TrimSpace(strings.ToLower(cfg.Profile.Mode))
	if mode != "pipeline_locked" && mode != "hop_override" {
		return fmt.Errorf("profile.mode 必須為 pipeline_locked 或 hop_override")
	}
	if cfg.Profile.Target != nil {
		t := cfg.Profile.Target
		if t.Width <= 0 || t.Height <= 0 {
			return fmt.Errorf("profile.target 之 width/height 必須為正整數")
		}
		if t.FrameRate <= 0 || t.FrameRate > 240 {
			return fmt.Errorf("profile.target.frame_rate 必須為合理正數（建議 ≤240）")
		}
	}
	for i, src := range cfg.Sources {
		slot := strings.TrimSpace(strings.ToLower(src.SlotID))
		if err := parseInputSlot(slot, inputs); err != nil {
			return fmt.Errorf("sources[%d].slot_id: %w", i, err)
		}
		if strings.TrimSpace(src.DisplayName) == "" {
			return fmt.Errorf("sources[%d].display_name 不可為空", i)
		}
	}
	seenSlot := map[string]struct{}{}
	for _, src := range cfg.Sources {
		slot := strings.TrimSpace(strings.ToLower(src.SlotID))
		if _, dup := seenSlot[slot]; dup {
			return fmt.Errorf("重複的 slot_id: %s", slot)
		}
		seenSlot[slot] = struct{}{}
	}
	if strings.TrimSpace(cfg.Switcher.PanelID) == "" {
		return fmt.Errorf("switcher.panel_id 不可為空")
	}
	for ri, row := range cfg.Switcher.Rows {
		for bi, btn := range row.Buttons {
			kind := strings.TrimSpace(strings.ToLower(btn.Bind.Kind))
			if kind != "input" && kind != "black" && kind != "bars" {
				return fmt.Errorf("switcher.rows[%d].buttons[%d]: bind.kind 必須為 input|black|bars", ri, bi)
			}
			if kind == "input" {
				ref := strings.TrimSpace(strings.ToLower(btn.Bind.Ref))
				if err := parseInputSlot(ref, inputs); err != nil {
					return fmt.Errorf("switcher.rows[%d].buttons[%d].bind.ref: %w", ri, bi, err)
				}
			}
		}
	}
	tpl := strings.TrimSpace(strings.ToLower(cfg.Multiview.TemplateID))
	if tpl == "" {
		return fmt.Errorf("multiview.template_id 不可為空")
	}
	for ci, cell := range cfg.Multiview.Cells {
		role := strings.TrimSpace(strings.ToLower(cell.Role))
		if role != "preview_large" && role != "program_large" && role != "thumb" && role != "off" {
			return fmt.Errorf("multiview.cells[%d].role 必須為 preview_large|program_large|thumb|off", ci)
		}
		ref := strings.TrimSpace(strings.ToLower(cell.SourceRef))
		if ref != "" && ref != "off" && !strings.HasPrefix(ref, "input") {
			return fmt.Errorf("multiview.cells[%d].source_ref 必須為空、off 或 inputN", ci)
		}
		if strings.HasPrefix(ref, "input") {
			if err := parseInputSlot(ref, inputs); err != nil {
				return fmt.Errorf("multiview.cells[%d].source_ref: %w", ci, err)
			}
		}
	}
	return nil
}

func parseInputSlot(slot string, maxInputs int) error {
	if !strings.HasPrefix(slot, "input") {
		return fmt.Errorf("必須為 input1..input%d 格式", maxInputs)
	}
	numStr := strings.TrimPrefix(slot, "input")
	n, err := strconv.Atoi(numStr)
	if err != nil || n < 1 || n > maxInputs {
		return fmt.Errorf("必須為 input1..input%d", maxInputs)
	}
	return nil
}
