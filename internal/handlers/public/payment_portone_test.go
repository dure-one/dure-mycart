package handlers

import (
	"testing"

	"github.com/shurco/mycart/pkg/logging"
)

func TestValidatePaymentAmount(t *testing.T) {
	t.Parallel()

	log := logging.New()

	tests := []struct {
		name         string
		paymentTotal int
		cartTotal    int
		wantErr      bool
	}{
		{
			name:         "valid amount match",
			paymentTotal: 100000, // 1000 USD in cents
			cartTotal:    1000,   // Cart total in cents
			wantErr:      false,
		},
		{
			name:         "amount mismatch",
			paymentTotal: 50000,
			cartTotal:    1000,
			wantErr:      true,
		},
		{
			name:         "zero amount",
			paymentTotal: 0,
			cartTotal:    0,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validatePaymentAmount(tt.paymentTotal, tt.cartTotal, log)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePaymentAmount() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePaymentCurrency(t *testing.T) {
	t.Parallel()

	log := logging.New()

	tests := []struct {
		name            string
		paymentCurrency string
		cartCurrency    string
		wantErr         bool
	}{
		{
			name:            "currency match",
			paymentCurrency: "USD",
			cartCurrency:    "USD",
			wantErr:         false,
		},
		{
			name:            "currency mismatch",
			paymentCurrency: "EUR",
			cartCurrency:    "USD",
			wantErr:         true,
		},
		{
			name:            "empty currencies",
			paymentCurrency: "",
			cartCurrency:    "",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validatePaymentCurrency(tt.paymentCurrency, tt.cartCurrency, log)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePaymentCurrency() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCartID(t *testing.T) {
	t.Parallel()

	log := logging.New()

	tests := []struct {
		name           string
		customDataJSON string
		expectedCartID string
		wantErr        bool
	}{
		{
			name:           "valid cart ID",
			customDataJSON: `{"cart_id":"cart123"}`,
			expectedCartID: "cart123",
			wantErr:        false,
		},
		{
			name:           "cart ID mismatch",
			customDataJSON: `{"cart_id":"cart123"}`,
			expectedCartID: "cart456",
			wantErr:        true,
		},
		{
			name:           "invalid JSON",
			customDataJSON: `{invalid}`,
			expectedCartID: "cart123",
			wantErr:        true,
		},
		{
			name:           "missing cart_id",
			customDataJSON: `{}`,
			expectedCartID: "cart123",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateCartID(tt.customDataJSON, tt.expectedCartID, log)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCartID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
