package queries

import (
	"strings"
	"testing"

	"github.com/shurco/mycart/internal/models"
)

// decodeJSONArray skips entries whose id decodes as empty. A real row always has
// an id, so the filter is defensive; these cases pin it and the aggregate shapes
// it has to tolerate.
func TestDecodeJSONArray(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantIDs []string
		wantErr string
	}{
		{"empty string", "", nil, ""},
		{"empty array", "[]", nil, ""},
		{
			name:    "entries with ids",
			raw:     `[{"id":"a"},{"id":"b"}]`,
			wantIDs: []string{"a", "b"},
		},
		{
			name:    "single null entry is dropped",
			raw:     `[{"id":null}]`,
			wantIDs: nil,
		},
		{
			name:    "null entries mixed with real ones",
			raw:     `[{"id":"a"},{"id":null},{"id":"b"}]`,
			wantIDs: []string{"a", "b"},
		},
		{
			// PostgreSQL spells an aggregate over no rows as a JSON null, not
			// as an array.
			name:    "json null",
			raw:     `null`,
			wantIDs: nil,
		},
		{"not json", `{"id":`, nil, "decode json array"},
		{"wrong shape", `{"id":"a"}`, nil, "decode json array"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []models.File
			err := decodeJSONArray(tt.raw, &got, func(f models.File) string { return f.ID })

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("decodeJSONArray: %v", err)
				}
			} else if err == nil {
				t.Fatal("expected an error")
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want it to mention %q", err, tt.wantErr)
			}

			var ids []string
			for _, f := range got {
				ids = append(ids, f.ID)
			}
			if len(ids) != len(tt.wantIDs) {
				t.Fatalf("ids = %v, want %v", ids, tt.wantIDs)
			}
			for i := range ids {
				if ids[i] != tt.wantIDs[i] {
					t.Fatalf("ids = %v, want %v", ids, tt.wantIDs)
				}
			}
		})
	}
}

// A "json null" aggregate must decode to a slice that is usable, not to a
// half-populated one: the callers append to what they are given.
func TestDecodeJSONArrayAppendsToWhatIsThere(t *testing.T) {
	existing := []models.File{{ID: "first"}}

	if err := decodeFiles(`[{"id":"second"}]`, &existing); err != nil {
		t.Fatalf("decodeFiles: %v", err)
	}
	if len(existing) != 2 || existing[0].ID != "first" || existing[1].ID != "second" {
		t.Errorf("got %+v", existing)
	}

	// And a null aggregate leaves it exactly as it was.
	if err := decodeFiles(`[{"id":null}]`, &existing); err != nil {
		t.Fatalf("decodeFiles: %v", err)
	}
	if len(existing) != 2 {
		t.Errorf("a null entry changed the slice: %+v", existing)
	}
}

func TestDecodeVariantsDropsNullEntries(t *testing.T) {
	var variants []models.ProductVariant

	if err := decodeVariants(`[{"id":"v1","sku":"sku-1"},{"id":null}]`, &variants); err != nil {
		t.Fatalf("decodeVariants: %v", err)
	}
	if len(variants) != 1 || variants[0].ID != "v1" || variants[0].SKU != "sku-1" {
		t.Errorf("got %+v", variants)
	}
}

func TestUnmarshalJSONToPointer(t *testing.T) {
	t.Run("empty value leaves the pointer alone", func(t *testing.T) {
		var ptr *models.NumberFormatSettings
		if err := unmarshalJSONToPointer("", &ptr); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if ptr != nil {
			t.Errorf("pointer = %+v, want nil", ptr)
		}
	})

	t.Run("valid json is decoded", func(t *testing.T) {
		var ptr *models.NumberFormatSettings
		if err := unmarshalJSONToPointer(`{"decimal_precision":2,"show_trailing_zeros":true}`, &ptr); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if ptr == nil || ptr.DecimalPrecision != 2 || !ptr.ShowTrailingZeros {
			t.Errorf("pointer = %+v", ptr)
		}
	})

	t.Run("broken json is reported", func(t *testing.T) {
		var ptr *models.NumberFormatSettings
		if err := unmarshalJSONToPointer("{not json", &ptr); err == nil {
			t.Error("expected an error")
		}
	})
}

func TestMarshalJSONFromPointer(t *testing.T) {
	t.Run("nil renders as empty", func(t *testing.T) {
		var ptr *models.NumberFormatSettings
		got, err := marshalJSONFromPointer(ptr)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want an empty string", got)
		}
	})

	t.Run("a value renders as json", func(t *testing.T) {
		got, err := marshalJSONFromPointer(&models.NumberFormatSettings{DecimalPrecision: 1})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(got, `"decimal_precision":1`) {
			t.Errorf("got %q", got)
		}
	})
}

// parseSettingValue is the settings form's decoder: every field type the admin
// UI can send has to round-trip, and a value that does not fit its type has to
// be an error rather than a silently zeroed field.
func TestParseSettingValue(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		var got string
		if err := parseSettingValue("hello", &got); err != nil || got != "hello" {
			t.Errorf("got %q (%v)", got, err)
		}
	})

	t.Run("bool", func(t *testing.T) {
		var got bool
		if err := parseSettingValue("true", &got); err != nil || !got {
			t.Errorf("got %v (%v)", got, err)
		}
		if err := parseSettingValue("not-a-bool", &got); err == nil {
			t.Error("expected an error for a value that is not a bool")
		}
	})

	t.Run("int", func(t *testing.T) {
		var got int
		if err := parseSettingValue("42", &got); err != nil || got != 42 {
			t.Errorf("got %d (%v)", got, err)
		}
		// An unset numeric setting is stored as an empty string, which means
		// zero rather than a parse failure.
		got = 7
		if err := parseSettingValue("", &got); err != nil || got != 0 {
			t.Errorf("empty value gave %d (%v), want 0", got, err)
		}
		if err := parseSettingValue("four", &got); err == nil {
			t.Error("expected an error for a value that is not an int")
		}
	})

	t.Run("json object", func(t *testing.T) {
		var got *models.SymbolDisplaySettings
		if err := parseSettingValue(`{"admin":"currency","storefront":"language"}`, &got); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got == nil || got.Admin != "currency" || got.Storefront != "language" {
			t.Errorf("got %+v", got)
		}
		if err := parseSettingValue("{broken", &got); err == nil {
			t.Error("expected an error for broken json")
		}
	})

	t.Run("unknown type is ignored", func(t *testing.T) {
		var got float64
		if err := parseSettingValue("1.5", &got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestSerializeSettingValue(t *testing.T) {
	tests := []struct {
		name      string
		value     any
		want      string
		wantKnown bool
	}{
		{"string", new(string), "", true},
		{"bool", new(bool), "false", true},
		{"int", new(int), "0", true},
		{"truncation json", new(*models.TruncationSettings), "", true},
		{"number format json", new(*models.NumberFormatSettings), "", true},
		{"symbol display json", new(*models.SymbolDisplaySettings), "", true},
		{"unknown type", new(float64), "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, known, err := serializeSettingValue(tt.value)
			if err != nil {
				t.Fatalf("serialize: %v", err)
			}
			if known != tt.wantKnown {
				t.Errorf("known = %v, want %v", known, tt.wantKnown)
			}
			if got != tt.want {
				t.Errorf("value = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("a populated json field is serialised", func(t *testing.T) {
		settings := &models.NumberFormatSettings{DecimalPrecision: 2, ShowTrailingZeros: true}
		ptr := &settings
		got, known, err := serializeSettingValue(ptr)
		if err != nil || !known {
			t.Fatalf("serialize: %q known=%v err=%v", got, known, err)
		}
		if !strings.Contains(got, `"decimal_precision":2`) {
			t.Errorf("got %q", got)
		}
	})

	// The two halves have to agree: what serialize writes is what parse reads.
	t.Run("round trip", func(t *testing.T) {
		original := &models.SymbolDisplaySettings{Admin: "language", Storefront: "currency"}
		ptr := &original

		text, _, err := serializeSettingValue(ptr)
		if err != nil {
			t.Fatalf("serialize: %v", err)
		}

		var back *models.SymbolDisplaySettings
		if err := parseSettingValue(text, &back); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if back == nil || *back != *original {
			t.Errorf("round trip gave %+v, want %+v", back, original)
		}
	})
}
