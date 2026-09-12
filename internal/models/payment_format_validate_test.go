package models

import (
	"strings"
	"testing"
)

func TestPayment_ValidateTruncation(t *testing.T) {
	t.Parallel()

	settings := func(mode, unit string) *TruncationSettings {
		return &TruncationSettings{
			Admin:      map[string]CurrencyTruncationSettings{"USD": {Mode: mode, FixedUnit: unit}},
			Storefront: map[string]CurrencyTruncationSettings{"USD": {Mode: mode, FixedUnit: unit}},
		}
	}

	tests := []struct {
		name    string
		trunc   *TruncationSettings
		wantErr string
	}{
		{"nil is allowed", nil, ""},
		{"mode none", settings("none", ""), ""},
		{"mode fixed with a unit", settings("fixed", "K"), ""},
		{"mode flexible", settings("flexible", ""), ""},
		{"mode fixed without a unit", settings("fixed", ""), "fixed_unit required when mode is 'fixed' for USD"},
		{"an unknown mode", settings("round", ""), "mode must be 'none', 'fixed', or 'flexible' for USD"},
		{"an empty mode", settings("", ""), "mode must be 'none', 'fixed', or 'flexible' for USD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (Payment{Currency: "USD", Truncation: tt.trunc}).Validate()
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("Validate() = %v, want nil", err)
			case tt.wantErr != "" && err == nil:
				t.Errorf("Validate() = nil, want %q", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Errorf("Validate() = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// The admin and the storefront are validated by the same loop, so a bad
// storefront mode must be rejected when the admin half is fine.
func TestPayment_ValidateTruncationStorefrontOnly(t *testing.T) {
	t.Parallel()

	payment := Payment{
		Currency: "USD",
		Truncation: &TruncationSettings{
			Admin:      map[string]CurrencyTruncationSettings{"USD": {Mode: "none"}},
			Storefront: map[string]CurrencyTruncationSettings{"USD": {Mode: "fixed"}},
		},
	}

	err := payment.Validate()
	if err == nil {
		t.Fatal("a fixed storefront mode without a unit must be rejected")
	}
	if !strings.Contains(err.Error(), "fixed_unit required") {
		t.Errorf("Validate() = %v, want it to name the missing unit", err)
	}
}

func TestValidateTruncationIgnoresOtherTypes(t *testing.T) {
	t.Parallel()

	// The rule runs against whatever the field holds, including a nil
	// interface and a value it did not expect.
	if err := validateTruncation(nil); err != nil {
		t.Errorf("validateTruncation(nil) = %v, want nil", err)
	}
	var typedNil *TruncationSettings
	if err := validateTruncation(typedNil); err != nil {
		t.Errorf("validateTruncation((*TruncationSettings)(nil)) = %v, want nil", err)
	}
	if err := validateTruncation("not truncation settings"); err != nil {
		t.Errorf("validateTruncation(string) = %v, want nil", err)
	}
}

func TestPayment_ValidateNumberFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		format  *NumberFormatSettings
		wantErr bool
	}{
		{"nil is allowed", nil, false},
		{"no decimals", &NumberFormatSettings{DecimalPrecision: 0}, false},
		{"one decimal", &NumberFormatSettings{DecimalPrecision: 1}, false},
		{"two decimals", &NumberFormatSettings{DecimalPrecision: 2}, false},
		{"negative precision", &NumberFormatSettings{DecimalPrecision: -1}, true},
		{"three decimals", &NumberFormatSettings{DecimalPrecision: 3}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (Payment{Currency: "USD", NumberFormat: tt.format}).Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "decimal_precision") {
				t.Errorf("Validate() = %v, want it to name decimal_precision", err)
			}
		})
	}
}

func TestPayment_ValidateSymbolDisplay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		display *SymbolDisplaySettings
		wantErr string
	}{
		{"nil is allowed", nil, ""},
		{"both empty", &SymbolDisplaySettings{}, ""},
		{"currency", &SymbolDisplaySettings{Admin: "currency", Storefront: "currency"}, ""},
		{"language", &SymbolDisplaySettings{Admin: "language", Storefront: "language"}, ""},
		{"bad admin mode", &SymbolDisplaySettings{Admin: "symbol"}, "admin symbol_display must be 'currency' or 'language'"},
		{"bad storefront mode", &SymbolDisplaySettings{Storefront: "symbol"}, "storefront symbol_display must be 'currency' or 'language'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (Payment{Currency: "USD", SymbolDisplay: tt.display}).Validate()
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("Validate() = %v, want nil", err)
			case tt.wantErr != "" && err == nil:
				t.Errorf("Validate() = nil, want %q", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Errorf("Validate() = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
