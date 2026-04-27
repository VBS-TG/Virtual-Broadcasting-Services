package showconfig

import "testing"

func TestValidate_DefaultInputs8(t *testing.T) {
	cfg := Default()
	Normalize(&cfg)
	if err := Validate(cfg, 8); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_SourceSlot_OutOfRange(t *testing.T) {
	cfg := Default()
	cfg.Sources = []SourceSlot{{SlotID: "input9", DisplayName: "x"}}
	Normalize(&cfg)
	if err := Validate(cfg, 8); err == nil {
		t.Fatal("expected error")
	}
}
