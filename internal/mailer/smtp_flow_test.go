package mailer

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime/quotedprintable"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
)

// fakeSMTP is as much of an SMTP server as go-simple-mail needs to connect,
// authenticate and hand over a message: the greeting, EHLO with the PLAIN
// mechanism, MAIL/RCPT, DATA and QUIT.
//
// It exists so the sending half of this package can be tested without a mail
// server, and so the tests can read back what was actually put on the wire.
type fakeSMTP struct {
	addr string

	mu       sync.Mutex
	messages []string
}

func newFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := &fakeSMTP{addr: ln.Addr().String()}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go server.serve(conn)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })

	return server
}

func (s *fakeSMTP) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	reader := bufio.NewReader(conn)
	reply := func(format string, args ...any) {
		_, _ = fmt.Fprintf(conn, format+"\r\n", args...)
	}

	reply("220 fake ESMTP ready")

	var body strings.Builder
	inData := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")

		if inData {
			if trimmed == "." {
				inData = false
				s.add(body.String())
				body.Reset()
				reply("250 2.0.0 Ok: queued")
				continue
			}
			body.WriteString(trimmed)
			body.WriteString("\n")
			continue
		}

		command := strings.ToUpper(trimmed)
		switch {
		case strings.HasPrefix(command, "EHLO"):
			reply("250-fake")
			// PLAIN only: the client then sends its credentials in a single
			// AUTH command instead of the LOGIN challenge dance.
			reply("250-AUTH PLAIN")
			reply("250 SIZE 10485760")
		case strings.HasPrefix(command, "HELO"):
			reply("250 fake")
		case strings.HasPrefix(command, "AUTH"):
			if len(strings.Fields(command)) >= 3 {
				reply("235 2.7.0 authenticated")
			} else {
				reply("334 ")
			}
		case strings.HasPrefix(command, "MAIL FROM"), strings.HasPrefix(command, "RCPT TO"):
			reply("250 2.1.0 Ok")
		case command == "DATA":
			inData = true
			reply("354 End data with <CR><LF>.<CR><LF>")
		case command == "QUIT":
			reply("221 2.0.0 Bye")
			return
		default:
			reply("250 2.0.0 Ok")
		}
	}
}

func (s *fakeSMTP) add(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, message)
}

// delivered returns the messages received so far.
func (s *fakeSMTP) delivered() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.messages...)
}

// decode returns the message split into its header block and the decoded body.
//
// go-simple-mail sends the body quoted-printable, which hides "=" as "=3D" and
// wraps long lines, so the raw text of a url or a price is not what is on the
// wire.
func decode(t *testing.T, message string) (headers, body string) {
	t.Helper()

	rawHeaders, rawBody, found := strings.Cut(message, "\n\n")
	if !found {
		t.Fatalf("message has no header/body separator:\n%s", message)
	}

	decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(rawBody)))
	if err != nil {
		t.Fatalf("decode quoted-printable body: %v", err)
	}
	return rawHeaders, string(decoded)
}

// expectOne asserts exactly one message arrived and returns it.
func (s *fakeSMTP) expectOne(t *testing.T) string {
	t.Helper()

	messages := s.delivered()
	if len(messages) != 1 {
		t.Fatalf("received %d messages, want 1: %v", len(messages), messages)
	}
	return messages[0]
}

// pointSettingsAtFake makes the stored mail settings use the fake server.
func pointSettingsAtFake(t *testing.T, server *fakeSMTP) {
	t.Helper()

	host, portStr, err := net.SplitHostPort(server.addr)
	if err != nil {
		t.Fatalf("split address: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	setting := &models.Mail{
		SenderName:  "Site Owner",
		SenderEmail: "owner@example.com",
		SMTP: models.SMTP{
			Host:       host,
			Port:       port,
			Username:   "smtp-user",
			Password:   "smtp-password",
			Encryption: "None",
		},
	}
	if err := queries.DB().UpdateSettingByGroup(context.Background(), setting); err != nil {
		t.Fatalf("point mail settings at the fake server: %v", err)
	}
}

func TestSendMailDelivers(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	server := newFakeSMTP(t)
	pointSettingsAtFake(t, server)

	setting, err := queries.GetSettingByGroup[models.Mail](context.Background(), queries.DB())
	if err != nil {
		t.Fatalf("load mail settings: %v", err)
	}

	err = SendMail(setting, &models.MessageMail{
		To: "buyer@example.com",
		Letter: models.Letter{
			Subject: "Your order",
			Text:    "Thanks {{.Name}}, your total is {{.Total}}.",
		},
		Data: map[string]string{"Name": "Sam", "Total": "21.00 USD"},
	})
	if err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	headers, body := decode(t, server.expectOne(t))
	for _, want := range []string{
		"Subject: Your order",
		"To: <buyer@example.com>",
		`From: "Site Owner" <owner@example.com>`,
	} {
		if !strings.Contains(headers, want) {
			t.Errorf("headers do not contain %q:\n%s", want, headers)
		}
	}
	if want := "Thanks Sam, your total is 21.00 USD."; !strings.Contains(body, want) {
		t.Errorf("body does not contain %q:\n%s", want, body)
	}
}

// The sender name is what makes the From header human; without one the address
// must stand alone rather than render as "<owner@example.com>".
func TestSendMailWithoutASenderName(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	server := newFakeSMTP(t)
	pointSettingsAtFake(t, server)

	setting, err := queries.GetSettingByGroup[models.Mail](context.Background(), queries.DB())
	if err != nil {
		t.Fatalf("load mail settings: %v", err)
	}
	setting.SenderName = ""

	if err := SendMail(setting, &models.MessageMail{
		To:     "buyer@example.com",
		Letter: models.Letter{Subject: "No name", Text: "body"},
	}); err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	headers, _ := decode(t, server.expectOne(t))
	if !strings.Contains(headers, "From: <owner@example.com>") {
		t.Errorf("headers do not carry the plain sender address:\n%s", headers)
	}
}

func TestSendMailAttachesFiles(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	// Attachments are read from ./lc_digitals, so the working directory has to
	// hold one — testutil gave us a temporary directory.
	if err := os.MkdirAll("./lc_digitals", 0o775); err != nil {
		t.Fatalf("create lc_digitals: %v", err)
	}
	if err := os.WriteFile("./lc_digitals/9f8e7d6c.dat", []byte("secret payload"), 0o644); err != nil {
		t.Fatalf("write digital file: %v", err)
	}

	server := newFakeSMTP(t)
	pointSettingsAtFake(t, server)

	setting, err := queries.GetSettingByGroup[models.Mail](context.Background(), queries.DB())
	if err != nil {
		t.Fatalf("load mail settings: %v", err)
	}

	err = SendMail(setting, &models.MessageMail{
		To:     "buyer@example.com",
		Letter: models.Letter{Subject: "Your download", Text: "attached"},
		Files:  []models.File{{Name: "9f8e7d6c", Ext: "dat", OrigName: "guide.pdf"}},
	})
	if err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	// Read the message as it went on the wire: the body is multipart, so it is
	// not quoted-printable as a whole and the attachment part is base64.
	message := server.expectOne(t)
	if !strings.Contains(message, `filename="guide.pdf"`) {
		t.Errorf("the attachment is missing the original filename:\n%s", message)
	}
	payload := base64.StdEncoding.EncodeToString([]byte("secret payload"))
	if !strings.Contains(message, payload) {
		t.Errorf("the attachment payload (%s) is missing:\n%s", payload, message)
	}
}

// A template that cannot be parsed must be reported, not swallowed and sent
// half-rendered.
func TestSendMailRejectsABrokenTemplate(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	server := newFakeSMTP(t)
	pointSettingsAtFake(t, server)

	setting, err := queries.GetSettingByGroup[models.Mail](context.Background(), queries.DB())
	if err != nil {
		t.Fatalf("load mail settings: %v", err)
	}

	err = SendMail(setting, &models.MessageMail{
		To:     "buyer@example.com",
		Letter: models.Letter{Subject: "Broken", Text: "Hello {{.Missing"},
	})
	if err == nil {
		t.Fatal("a template that does not parse must be an error")
	}
	if got := len(server.delivered()); got != 0 {
		t.Errorf("%d messages were sent despite the broken template", got)
	}
}

func TestSendTestLetter(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	server := newFakeSMTP(t)
	pointSettingsAtFake(t, server)

	if err := SendTestLetter("smtp"); err != nil {
		t.Fatalf("SendTestLetter(smtp): %v", err)
	}

	headers, _ := decode(t, server.expectOne(t))
	if !strings.Contains(strings.ToLower(headers), "mycart test smtp settings") {
		t.Errorf("headers do not carry the test subject:\n%s", headers)
	}
	// The recipient is the site's own address.
	if !strings.Contains(headers, "user@mail.com") {
		t.Errorf("message does not go to the site address:\n%s", headers)
	}
}

// Any letter name other than "smtp" is a stored template, and is sent as it is
// stored rather than as the built-in test message.
func TestSendTestLetterUsesTheStoredTemplate(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	server := newFakeSMTP(t)
	pointSettingsAtFake(t, server)

	if err := SendTestLetter("mail_letter_payment"); err != nil {
		t.Fatalf("SendTestLetter(mail_letter_payment): %v", err)
	}

	headers, _ := decode(t, server.expectOne(t))
	if !strings.Contains(headers, "New payment transaction") {
		t.Errorf("headers do not carry the stored subject:\n%s", headers)
	}
}

func TestSendPrepaymentLetter(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	server := newFakeSMTP(t)
	pointSettingsAtFake(t, server)

	paymentURL := "https://site.com/cart/payment?cart_id=abc"
	if err := SendPrepaymentLetter("buyer@example.com", "21.00 USD", paymentURL); err != nil {
		t.Fatalf("SendPrepaymentLetter: %v", err)
	}

	headers, body := decode(t, server.expectOne(t))
	if !strings.Contains(headers, "New payment transaction") {
		t.Errorf("headers do not carry the stored subject:\n%s", headers)
	}
	for _, want := range []string{"21.00 USD", paymentURL, "[Site name]"} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q:\n%s", want, body)
		}
	}
}

// stageDigitalFiles writes the fixture digital files into ./lc_digitals, which
// is where the purchase letter reads attachments from. Without them the letter
// fails on the first file rather than sending, which is the behaviour a real
// installation would see only if its upload directory had been wiped.
func stageDigitalFiles(t *testing.T) {
	t.Helper()

	rows, err := queries.DB().ProductQueries.DB.QueryContext(context.Background(),
		`SELECT name, ext FROM digital_file`)
	if err != nil {
		t.Fatalf("list digital files: %v", err)
	}
	defer func() { _ = rows.Close() }()

	if err := os.MkdirAll("./lc_digitals", 0o775); err != nil {
		t.Fatalf("create lc_digitals: %v", err)
	}

	staged := 0
	for rows.Next() {
		var name, ext string
		if err := rows.Scan(&name, &ext); err != nil {
			t.Fatalf("scan digital file: %v", err)
		}
		path := fmt.Sprintf("./lc_digitals/%s.%s", name, ext)
		if err := os.WriteFile(path, []byte("fixture payload for "+name), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		staged++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate digital files: %v", err)
	}
	if staged == 0 {
		t.Fatal("the fixtures carry no digital files to stage")
	}
}

func TestSendCartLetter(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))
	stageDigitalFiles(t)

	server := newFakeSMTP(t)
	pointSettingsAtFake(t, server)

	// A paid cart from the fixtures.
	if err := SendCartLetter("iodz4ibf5h5zmov"); err != nil {
		t.Fatalf("SendCartLetter: %v", err)
	}

	headers, body := decode(t, server.expectOne(t))
	if !strings.Contains(headers, "user@gmail.com") {
		t.Errorf("the purchase letter does not go to the buyer:\n%s", headers)
	}
	// The fixtures grant a digital product, so the purchase letter carries its
	// download key or data.
	if !strings.Contains(body, "Best regards") && !strings.Contains(headers, "attachment") {
		t.Errorf("the purchase letter carries neither the template body nor an attachment:\n%s\n%s", headers, body)
	}
}

// A purchase letter for a cart whose product delivers nothing still sends: there
// is simply nothing to attach. ('api' is the third digital type the schema
// allows, and the one with neither files nor keys.)
func TestSendCartLetterWithoutDigitalContent(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	server := newFakeSMTP(t)
	pointSettingsAtFake(t, server)

	ctx := context.Background()
	var productID string
	if err := queries.DB().ProductQueries.DB.QueryRowContext(ctx,
		`SELECT id FROM product ORDER BY id LIMIT 1`).Scan(&productID); err != nil {
		t.Fatalf("pick a product: %v", err)
	}
	if _, err := queries.DB().ProductQueries.DB.ExecContext(ctx,
		`UPDATE product SET digital = 'api' WHERE id = ?`, productID); err != nil {
		t.Fatalf("give the product a digital type with nothing to deliver: %v", err)
	}

	// The cart column holds the product list as JSON.
	cartJSON := fmt.Sprintf(`[{"id":%q,"quantity":1}]`, productID)
	if _, err := queries.DB().CartQueries.DB.ExecContext(ctx,
		`UPDATE cart SET cart = ? WHERE id = ?`, cartJSON, "iodz4ibf5h5zmov"); err != nil {
		t.Fatalf("rewrite the cart's product list: %v", err)
	}

	if err := SendCartLetter("iodz4ibf5h5zmov"); err != nil {
		t.Fatalf("SendCartLetter: %v", err)
	}
	server.expectOne(t)
}
