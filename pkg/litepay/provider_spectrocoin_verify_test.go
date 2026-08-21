package litepay

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/url"
	"testing"
)

// signCallback produces a valid SpectroCoin-style signature for the callback
// using the given private key, mirroring the documented signing procedure.
func signCallback(t *testing.T, key *rsa.PrivateKey, cb *CallbackSpectrocoin) string {
	t.Helper()

	hash := sha1.Sum([]byte(callbackSignString(cb)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, hash[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return base64.StdEncoding.EncodeToString(signature)
}

// publicKeyPEM marshals an RSA public key to PKIX PEM, as used by the
// embedded SpectroCoin key.
func publicKeyPEM(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()

	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func TestVerifySpectrocoinCallback_HappyPath(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	cb := &CallbackSpectrocoin{
		MerchantID:      12345,
		ApiID:           1,
		UserID:          "user-1",
		MerchantApiID:   "api-1",
		OrderID:         "ABC123",
		PayCurrency:     "BTC",
		PayAmount:       0.025,
		ReceiveCurrency: "EUR",
		ReceiveAmount:   245.5,
		ReceivedAmount:  245,
		Description:     "Some string with symbols %=& +",
		OrderRequestID:  11,
		Status:          3,
	}
	cb.Sign = signCallback(t, key, cb)

	if err := verifySpectrocoinCallback(publicKeyPEM(t, key), cb); err != nil {
		t.Fatalf("expected valid signature, got error: %v", err)
	}
}

func TestVerifySpectrocoinCallback_TamperedStatus(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	cb := &CallbackSpectrocoin{
		MerchantID:      12345,
		ApiID:           1,
		OrderID:         "ABC123",
		PayCurrency:     "BTC",
		PayAmount:       1.0,
		ReceiveCurrency: "EUR",
		ReceiveAmount:   10.0,
		Description:     "",
		OrderRequestID:  11,
		Status:          1,
	}
	cb.Sign = signCallback(t, key, cb)

	// Attacker flips status 1 (new) to 3 (paid) without re-signing.
	cb.Status = 3

	if err := verifySpectrocoinCallback(publicKeyPEM(t, key), cb); err == nil {
		t.Fatal("expected signature verification to fail on tampered status")
	}
}

func TestVerifySpectrocoinCallback_WrongKeyAndEmptySign(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	cb := &CallbackSpectrocoin{
		MerchantID:      1,
		ApiID:           1,
		OrderID:         "X",
		PayCurrency:     "BTC",
		PayAmount:       1.0,
		ReceiveCurrency: "EUR",
		ReceiveAmount:   1.0,
		OrderRequestID:  1,
		Status:          3,
	}
	cb.Sign = signCallback(t, other, cb)
	if err := verifySpectrocoinCallback(publicKeyPEM(t, key), cb); err == nil {
		t.Fatal("signature from a foreign key must not validate")
	}

	cb.Sign = ""
	if err := VerifySpectrocoinCallback(cb); err == nil {
		t.Fatal("empty signature must be rejected")
	}
}

func TestCallbackSignString_Formatting(t *testing.T) {
	cb := &CallbackSpectrocoin{
		MerchantID:      25,
		ApiID:           25,
		OrderID:         "L254S",
		PayCurrency:     "BTC",
		PayAmount:       25,
		ReceiveCurrency: "EUR",
		ReceiveAmount:   245,
		Description:     `Some sting with symbols %=&`,
		OrderRequestID:  11,
		Status:          1,
	}

	want := "merchantId=25&apiId=25&orderId=L254S&payCurrency=BTC&payAmount=25.0&receiveCurrency=EUR&receiveAmount=245.0" +
		"&description=" + url.QueryEscape(`Some sting with symbols %=&`) +
		"&orderRequestId=11&status=1"

	if got := callbackSignString(cb); got != want {
		t.Errorf("callbackSignString()\n got: %s\nwant: %s", got, want)
	}
}

func TestSpectrocoinAmount(t *testing.T) {
	cases := map[float64]string{
		25:      "25.0",
		245:     "245.0",
		123.45:  "123.45",
		0.025:   "0.025",
		1.23456: "1.23456",
	}
	for in, want := range cases {
		if got := spectrocoinAmount(in); got != want {
			t.Errorf("spectrocoinAmount(%v) = %q, want %q", in, got, want)
		}
	}
}

// compile-time guard that the embedded public key stays parseable
func TestEmbeddedPublicKeyParses(t *testing.T) {
	block, _ := pem.Decode([]byte(spectroCoinPublicKey))
	if block == nil {
		t.Fatal("embedded PEM does not decode")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}
	if _, ok := parsed.(*rsa.PublicKey); !ok {
		t.Fatalf("embedded key is %T, want *rsa.PublicKey", parsed)
	}
}
