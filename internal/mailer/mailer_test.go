package mailer

import (
	"context"
	"strings"
	"testing"

	mailer "github.com/xhit/go-simple-mail/v2"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store"
	"github.com/shurco/mycart/internal/testutil"
)


func TestEncryptionTypesLookup(t *testing.T) {
	t.Parallel()
	if EncryptionTypes["None"] != mailer.EncryptionNone {
		t.Error("missing None encryption")
	}
	if EncryptionTypes["SSL/TLS"] != mailer.EncryptionSSL {
		t.Error("missing SSL/TLS encryption")
	}
	if EncryptionTypes["STARTTLS"] != mailer.EncryptionTLS {
		t.Error("missing STARTTLS encryption")
	}
}

func TestSendMail_ValidatesSMTPFields(t *testing.T) {
	t.Parallel()

	// Missing host should short-circuit before any network traffic.
	setting := &models.Mail{SenderEmail: "no@reply"}
	err := SendMail(setting, &models.MessageMail{To: "x@example.com"})
	if err == nil || !strings.Contains(err.Error(), "SMTP settings") {
		t.Fatalf("expected SMTP validation error, got %v", err)
	}
}

func TestSendMail_RequiresSenderEmail(t *testing.T) {
	t.Parallel()

	setting := &models.Mail{}
	setting.SMTP.Host = "localhost"
	setting.SMTP.Port = 25
	setting.SMTP.Username = "u"
	setting.SMTP.Password = "p"

	err := SendMail(setting, &models.MessageMail{To: "to@example.com"})
	if err == nil || !strings.Contains(err.Error(), "sender email") {
		t.Fatalf("expected sender email error, got %v", err)
	}
}

func TestSendMail_UnreachableServer(t *testing.T) {
	t.Parallel()

	setting := &models.Mail{
		SenderEmail: "a@b.com",
	}
	setting.SMTP.Host = "127.0.0.1"
	setting.SMTP.Port = 1 // reserved; connections refused immediately
	setting.SMTP.Username = "u"
	setting.SMTP.Password = "p"

	err := SendMail(setting, &models.MessageMail{
		To:     "to@example.com",
		Letter: models.Letter{Subject: "s", Text: "body"},
	})
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestTextTemplate_RendersData(t *testing.T) {
	t.Parallel()

	out, err := textTemplate("Hello {{.Name}}", map[string]string{"Name": "world"})
	if err != nil {
		t.Fatalf("textTemplate: %v", err)
	}
	if string(out) != "Hello world" {
		t.Errorf("got %q, want %q", out, "Hello world")
	}

	if _, err := textTemplate("{{.}", nil); err == nil {
		t.Error("expected parse error for malformed template")
	}
}

func TestEnsureSenderEmail_UsesConfigured(t *testing.T) {
	_, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	ctx := context.Background()
	m := &models.Mail{SenderEmail: "configured@example.com"}
	if err := ensureSenderEmail(ctx, m); err != nil {
		t.Fatalf("ensureSenderEmail: %v", err)
	}
	if m.SenderEmail != "configured@example.com" {
		t.Errorf("sender mutated unexpectedly: %s", m.SenderEmail)
	}
}

func TestEnsureSenderEmail_EmptyFallbackMissing(t *testing.T) {
	_, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	ctx := context.Background()

	// With a blank `email` setting the function must surface an error rather
	// than silently leaving SenderEmail blank.
	m := &models.Mail{}
	if err := ensureSenderEmail(ctx, m); err == nil {
		t.Error("expected error when both sender email and user email are empty")
	}
}

func TestEnsureSenderEmail_FallbackSuccess(t *testing.T) {
	_, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	ctx := context.Background()

	if err := store.UpdateSettingByKey(ctx, &models.SettingName{
		Key:   "email",
		Value: "owner@example.com",
	}); err != nil {
		t.Fatalf("seed email: %v", err)
	}

	m := &models.Mail{}
	if err := ensureSenderEmail(ctx, m); err != nil {
		t.Fatalf("ensureSenderEmail: %v", err)
	}
	if m.SenderEmail != "owner@example.com" {
		t.Errorf("fallback not applied: %+v", m)
	}
}

func TestSendTestLetter_RejectsMissingSMTP(t *testing.T) {
	_, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	if err := SendTestLetter("smtp"); err == nil {
		t.Error("expected error when SMTP is unconfigured")
	}
}

// seedSMTP fills the mail settings group with a minimal usable SMTP config,
// so letter functions proceed past the graceful "SMTP not configured" skip.
func seedSMTP(t *testing.T) {
	t.Helper()

	setting, err := store.GetSettingByGroupTyped[models.Mail](context.Background())
	if err != nil {
		t.Fatalf("load mail settings: %v", err)
	}
	setting.SMTP.Host = "127.0.0.1"
	setting.SMTP.Port = 25
	setting.SMTP.Username = "u"
	setting.SMTP.Password = "p"
	if err := store.UpdateSettingByGroup(context.Background(), setting); err != nil {
		t.Fatalf("seed smtp: %v", err)
	}
}

func TestSendPrepaymentLetter_MissingTemplate(t *testing.T) {
	_, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	seedSMTP(t)
	// With SMTP configured the call reaches CartLetterPayment, which fails
	// to unmarshal the missing payment template.
	if err := SendPrepaymentLetter("x@y.com", "1 USD", "http://pay"); err == nil {
		t.Error("expected error for missing payment template")
	}
}

func TestSendPrepaymentLetter_SkipsWhenSMTPUnconfigured(t *testing.T) {
	_, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// No template seeded and no SMTP configured: the letter must be skipped
	// gracefully instead of returning an error (dev/test environments).
	if err := SendPrepaymentLetter("x@y.com", "1 USD", "http://pay"); err != nil {
		t.Errorf("expected nil error when SMTP is unconfigured, got %v", err)
	}
}

func TestSendCartLetter_CartNotFound(t *testing.T) {
	_, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	seedSMTP(t)

	if err := SendCartLetter("missing-cart"); err == nil {
		t.Error("expected error for missing cart")
	}
}
