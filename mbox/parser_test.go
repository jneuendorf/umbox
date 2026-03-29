package mbox

// Go test files must:
//   - Live in the same package as the code they test (or packagename_test for black-box tests)
//   - Have a filename ending in _test.go
//   - Contain functions named TestXxx(t *testing.T)
//
// Run tests with: go test ./mbox/ -v
// The -v flag shows verbose output (each test name + PASS/FAIL).

import (
	"fmt"
	"mime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Test data — these structs define emails AND the expected parse results.
// The generateMbox helper builds valid mbox content from them, so we're
// always comparing against the same source of truth.
// ---------------------------------------------------------------------------

// testEmail holds everything needed to generate one email in an mbox file
// and to verify the parsed result.
type testEmail struct {
	From        string   // decoded sender (what we expect after parsing)
	To          []string // decoded recipients
	Subject     string   // decoded subject
	Body        string   // plain text body
	HTMLBody    string   // optional HTML body (used in multipart/alternative)
	Attachments []testAttachment
	MIMEEncode  bool // if true, From/Subject headers will be MIME-encoded (RFC 2047)
}

type testAttachment struct {
	Filename    string
	ContentType string
	Data        string // plain text content (will be included as-is, not base64)
}

// testEmails is the canonical test dataset. Tests reference this directly.
var testEmails = []testEmail{
	{
		// Email 0: Simple plain text, ASCII only.
		From:    "Alice <alice@example.com>",
		To:      []string{"Bob <bob@example.com>"},
		Subject: "Hello from umbox!",
		Body:    "Hey Bob,\n\nThis is a test email.\n\nBest,\nAlice",
	},
	{
		// Email 1: Multiple recipients.
		From:    "Charlie <charlie@example.com>",
		To:      []string{"Alice <alice@example.com>", "Bob <bob@example.com>"},
		Subject: "Re: Meeting notes",
		Body:    "Hi everyone,\n\nHere are the meeting notes.",
	},
	{
		// Email 2: Multipart with attachment.
		From:    "Dave <dave@example.com>",
		To:      []string{"team@example.com"},
		Subject: "Quarterly Report",
		Body:    "Hi team,\n\nPlease find the report attached.",
		Attachments: []testAttachment{
			{Filename: "report.pdf", ContentType: "application/pdf", Data: "fake-pdf-content"},
		},
	},
	{
		// Email 3: MIME-encoded headers with non-ASCII characters.
		// This tests RFC 2047 decoding (the bug with "Alumni Büro" etc.).
		From:       "Alumni Büro der Freien Universität Berlin <alumni@fu-berlin.de>",
		To:         []string{"student@example.com"},
		Subject:    "Einladung in das Alumni-Netzwerk der Freien Universität Berlin",
		Body:       "Sehr geehrte Damen und Herren,\n\nwir laden Sie herzlich ein.",
		MIMEEncode: true,
	},
	{
		// Email 4: MIME-encoded with Japanese characters.
		From:       "田中太郎 <tanaka@example.jp>",
		To:         []string{"user@example.com"},
		Subject:    "会議のお知らせ",
		Body:       "明日の会議についてお知らせします。",
		MIMEEncode: true,
	},
	{
		// Email 5: HTML-only email (no plain text part).
		From:    "newsletter@example.com",
		To:      []string{"subscriber@example.com"},
		Subject: "Weekly Newsletter",
		HTMLBody: "<html><body><h1>Newsletter</h1><p>Hello subscriber!</p></body></html>",
	},
}

// ---------------------------------------------------------------------------
// Mbox generator — builds valid mbox content from testEmail structs.
// ---------------------------------------------------------------------------

// generateMbox creates a complete mbox file from the test email definitions.
// By generating the mbox from the same structs we assert against, we guarantee
// that expected values and mbox content are always in sync.
func generateMbox(emails []testEmail) string {
	var b strings.Builder

	for i, e := range emails {
		// Mbox separator line. The format is "From <sender> <date>".
		fmt.Fprintf(&b, "From sender%d@example.com Sat Mar 29 %02d:00:00 2025\n", i, 10+i)

		// Encode headers if MIMEEncode is set, otherwise use raw values.
		from := e.From
		subject := e.Subject
		if e.MIMEEncode {
			from = mimeEncode(e.From)
			subject = mimeEncode(e.Subject)
		}

		fmt.Fprintf(&b, "From: %s\n", from)
		fmt.Fprintf(&b, "To: %s\n", strings.Join(encodeTo(e.To, e.MIMEEncode), ", "))
		fmt.Fprintf(&b, "Subject: %s\n", subject)
		fmt.Fprintf(&b, "Date: Sat, 29 Mar 2025 %02d:00:00 +0000\n", 10+i)
		fmt.Fprintf(&b, "Message-ID: <msg%03d@example.com>\n", i+1)
		fmt.Fprintf(&b, "MIME-Version: 1.0\n")

		// Choose content structure based on whether we have attachments or HTML.
		switch {
		case len(e.Attachments) > 0:
			writeMixedMultipart(&b, e)
		case e.HTMLBody != "" && e.Body == "":
			// HTML-only email.
			fmt.Fprintf(&b, "Content-Type: text/html; charset=\"utf-8\"\n\n")
			b.WriteString(e.HTMLBody)
			b.WriteByte('\n')
		default:
			// Simple plain text email.
			fmt.Fprintf(&b, "Content-Type: text/plain; charset=\"utf-8\"\n\n")
			b.WriteString(e.Body)
			b.WriteByte('\n')
		}

		b.WriteByte('\n') // blank line between messages
	}

	return b.String()
}

// mimeEncode encodes a string using RFC 2047 Q-encoding (quoted-printable).
// For addresses like "Name <addr>", only the name part is encoded.
func mimeEncode(s string) string {
	// Check if this is an email address like "Name <addr>".
	if idx := strings.LastIndex(s, " <"); idx != -1 {
		name := s[:idx]
		addr := s[idx:]
		encoded := mime.QEncoding.Encode("utf-8", name)
		return encoded + addr
	}
	return mime.QEncoding.Encode("utf-8", s)
}

// encodeTo encodes a slice of To addresses.
func encodeTo(addrs []string, doEncode bool) []string {
	if !doEncode {
		return addrs
	}
	out := make([]string, len(addrs))
	for i, a := range addrs {
		out[i] = mimeEncode(a)
	}
	return out
}

// writeMixedMultipart writes a multipart/mixed email with text body + attachments.
func writeMixedMultipart(b *strings.Builder, e testEmail) {
	boundary := "----=_TestBoundary"
	fmt.Fprintf(b, "Content-Type: multipart/mixed; boundary=\"%s\"\n\n", boundary)

	// Text part.
	fmt.Fprintf(b, "--%s\n", boundary)
	fmt.Fprintf(b, "Content-Type: text/plain; charset=\"utf-8\"\n\n")
	b.WriteString(e.Body)
	b.WriteByte('\n')

	// Attachment parts.
	for _, att := range e.Attachments {
		fmt.Fprintf(b, "\n--%s\n", boundary)
		fmt.Fprintf(b, "Content-Type: %s; name=\"%s\"\n", att.ContentType, att.Filename)
		fmt.Fprintf(b, "Content-Disposition: attachment; filename=\"%s\"\n\n", att.Filename)
		b.WriteString(att.Data)
		b.WriteByte('\n')
	}

	fmt.Fprintf(b, "\n--%s--\n", boundary)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestParseMessageCount verifies that the parser finds the correct number of
// emails in the generated mbox.
func TestParseMessageCount(t *testing.T) {
	mboxData := generateMbox(testEmails)
	messages, err := ParseReader(strings.NewReader(mboxData))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}

	if got, want := len(messages), len(testEmails); got != want {
		t.Errorf("message count = %d, want %d", got, want)
	}
}

// TestParseSubjects checks that each email's subject is correctly parsed.
// This also verifies RFC 2047 MIME decoding for non-ASCII subjects.
func TestParseSubjects(t *testing.T) {
	mboxData := generateMbox(testEmails)
	messages, err := ParseReader(strings.NewReader(mboxData))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}

	for i, want := range testEmails {
		if i >= len(messages) {
			t.Errorf("message %d: missing (only %d messages parsed)", i, len(messages))
			continue
		}
		if got := messages[i].Subject; got != want.Subject {
			t.Errorf("message %d subject:\n  got  = %q\n  want = %q", i, got, want.Subject)
		}
	}
}

// TestParseFrom checks that the From header is correctly parsed and decoded.
func TestParseFrom(t *testing.T) {
	mboxData := generateMbox(testEmails)
	messages, err := ParseReader(strings.NewReader(mboxData))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}

	for i, want := range testEmails {
		if i >= len(messages) {
			t.Errorf("message %d: missing", i)
			continue
		}
		if got := messages[i].From; got != want.From {
			t.Errorf("message %d from:\n  got  = %q\n  want = %q", i, got, want.From)
		}
	}
}

// TestParseTo checks that the To header is correctly split and decoded.
func TestParseTo(t *testing.T) {
	mboxData := generateMbox(testEmails)
	messages, err := ParseReader(strings.NewReader(mboxData))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}

	for i, want := range testEmails {
		if i >= len(messages) {
			t.Errorf("message %d: missing", i)
			continue
		}
		got := messages[i].To
		if len(got) != len(want.To) {
			t.Errorf("message %d To count = %d, want %d", i, len(got), len(want.To))
			continue
		}
		for j := range want.To {
			if got[j] != want.To[j] {
				t.Errorf("message %d To[%d]:\n  got  = %q\n  want = %q", i, j, got[j], want.To[j])
			}
		}
	}
}

// TestParseBody checks that the plain text body is correctly extracted.
func TestParseBody(t *testing.T) {
	mboxData := generateMbox(testEmails)
	messages, err := ParseReader(strings.NewReader(mboxData))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}

	for i, want := range testEmails {
		if i >= len(messages) {
			t.Errorf("message %d: missing", i)
			continue
		}
		if want.Body != "" {
			// Trim trailing whitespace — the parser may include trailing newlines.
			got := strings.TrimRight(messages[i].TextBody, "\n\r ")
			expected := strings.TrimRight(want.Body, "\n\r ")
			if got != expected {
				t.Errorf("message %d body:\n  got  = %q\n  want = %q", i, got, expected)
			}
		}
	}
}

// TestParseHTMLBody checks that HTML-only emails have their body in HTMLBody.
func TestParseHTMLBody(t *testing.T) {
	mboxData := generateMbox(testEmails)
	messages, err := ParseReader(strings.NewReader(mboxData))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}

	for i, want := range testEmails {
		if i >= len(messages) {
			continue
		}
		if want.HTMLBody != "" && want.Body == "" {
			got := strings.TrimRight(messages[i].HTMLBody, "\n\r ")
			expected := strings.TrimRight(want.HTMLBody, "\n\r ")
			if got != expected {
				t.Errorf("message %d HTML body:\n  got  = %q\n  want = %q", i, got, expected)
			}
			// TextBody should be empty for HTML-only emails.
			if messages[i].TextBody != "" {
				t.Errorf("message %d: expected empty TextBody for HTML-only email, got %q",
					i, messages[i].TextBody)
			}
		}
	}
}

// TestParseAttachments verifies that attachments are detected with the correct
// filename and content type.
func TestParseAttachments(t *testing.T) {
	mboxData := generateMbox(testEmails)
	messages, err := ParseReader(strings.NewReader(mboxData))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}

	for i, want := range testEmails {
		if i >= len(messages) {
			continue
		}
		got := messages[i].Attachments

		if len(got) != len(want.Attachments) {
			t.Errorf("message %d attachment count = %d, want %d", i, len(got), len(want.Attachments))
			continue
		}

		for j, wantAtt := range want.Attachments {
			if got[j].Filename != wantAtt.Filename {
				t.Errorf("message %d attachment %d filename = %q, want %q",
					i, j, got[j].Filename, wantAtt.Filename)
			}
			if !strings.Contains(got[j].ContentType, wantAtt.ContentType) {
				t.Errorf("message %d attachment %d content type = %q, want to contain %q",
					i, j, got[j].ContentType, wantAtt.ContentType)
			}
		}
	}
}

// TestHasAttachments verifies the HasAttachments helper method.
func TestHasAttachments(t *testing.T) {
	mboxData := generateMbox(testEmails)
	messages, err := ParseReader(strings.NewReader(mboxData))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}

	for i, want := range testEmails {
		if i >= len(messages) {
			continue
		}
		expected := len(want.Attachments) > 0
		if got := messages[i].HasAttachments(); got != expected {
			t.Errorf("message %d HasAttachments() = %v, want %v", i, got, expected)
		}
	}
}

// TestMIMEDecodingSpecific tests RFC 2047 decoding in detail by generating
// an mbox with known encoded values and verifying the decoded output.
func TestMIMEDecodingSpecific(t *testing.T) {
	// These test cases exercise different MIME encoding scenarios.
	cases := []testEmail{
		{
			From:       "Ärzte ohne Grenzen <info@aerzte.org>",
			To:         []string{"spender@example.de"},
			Subject:    "Spendenbescheinigung für 2025",
			Body:       "Vielen Dank!",
			MIMEEncode: true,
		},
		{
			From:       "François Müller <francois@example.ch>",
			To:         []string{"büro@example.ch"},
			Subject:    "Réservation confirmée — Zürich → Genève",
			Body:       "Confirmation de votre réservation.",
			MIMEEncode: true,
		},
	}

	mboxData := generateMbox(cases)
	messages, err := ParseReader(strings.NewReader(mboxData))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}

	if len(messages) != len(cases) {
		t.Fatalf("got %d messages, want %d", len(messages), len(cases))
	}

	for i, want := range cases {
		msg := messages[i]
		if msg.From != want.From {
			t.Errorf("case %d From:\n  got  = %q\n  want = %q", i, msg.From, want.From)
		}
		if msg.Subject != want.Subject {
			t.Errorf("case %d Subject:\n  got  = %q\n  want = %q", i, msg.Subject, want.Subject)
		}
	}
}

// TestParseEmptyMbox verifies that an empty input returns zero messages.
func TestParseEmptyMbox(t *testing.T) {
	messages, err := ParseReader(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages from empty mbox, got %d", len(messages))
	}
}

// TestParseSingleMessage verifies parsing works with just one email.
func TestParseSingleMessage(t *testing.T) {
	single := []testEmail{testEmails[0]}
	mboxData := generateMbox(single)
	messages, err := ParseReader(strings.NewReader(mboxData))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Subject != single[0].Subject {
		t.Errorf("subject = %q, want %q", messages[0].Subject, single[0].Subject)
	}
}
