package mbox

// Go test files must:
//   - Live in the same package as the code they test (or packagename_test for black-box tests)
//   - Have a filename ending in _test.go
//   - Contain functions named TestXxx(t *testing.T)
//
// Run tests with: go test ./mbox/ -v
// The -v flag shows verbose output (each test name + PASS/FAIL).

import (
	"encoding/base64"
	"fmt"
	"mime"
	"strings"
	"testing"
	"time"
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
	Data        string // the expected decoded content
	Encoding    string // "base64", "quoted-printable", or "" for no encoding
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
	{
		// Email 6: Multipart with base64-encoded attachment.
		// This verifies that Content-Transfer-Encoding: base64 is decoded
		// correctly, so binary attachments (images, PDFs) are not corrupted.
		From:    "Eve <eve@example.com>",
		To:      []string{"team@example.com"},
		Subject: "Photo from the event",
		Body:    "See the attached photo.",
		Attachments: []testAttachment{
			{
				Filename:    "photo.png",
				ContentType: "image/png",
				Data:        "PNG\x89\x50\x4e\x47\x0d\x0a\x1a\x0abinary-image-data\x00\xff",
				Encoding:    "base64",
			},
		},
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
		fmt.Fprintf(b, "Content-Disposition: attachment; filename=\"%s\"\n", att.Filename)

		if att.Encoding != "" {
			fmt.Fprintf(b, "Content-Transfer-Encoding: %s\n", att.Encoding)
		}
		b.WriteByte('\n') // blank line after headers

		// Write the body: encode it if the test specifies an encoding.
		switch att.Encoding {
		case "base64":
			encoded := base64.StdEncoding.EncodeToString([]byte(att.Data))
			// Split into 76-char lines (standard for email base64).
			for i := 0; i < len(encoded); i += 76 {
				end := i + 76
				if end > len(encoded) {
					end = len(encoded)
				}
				b.WriteString(encoded[i:end])
				b.WriteByte('\n')
			}
		default:
			b.WriteString(att.Data)
			b.WriteByte('\n')
		}
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

// ---------------------------------------------------------------------------
// Content-Transfer-Encoding tests
// ---------------------------------------------------------------------------

// TestParseBase64Attachment verifies that base64-encoded attachments are
// decoded to the original binary data. This is the fix for corrupted images.
func TestParseBase64Attachment(t *testing.T) {
	// Find the base64-encoded attachment test case (email 6).
	email := testEmails[6]
	mboxData := generateMbox([]testEmail{email})
	messages, err := ParseReader(strings.NewReader(mboxData))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	msg := messages[0]
	if len(msg.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(msg.Attachments))
	}

	att := msg.Attachments[0]
	wantData := email.Attachments[0].Data

	if att.Filename != "photo.png" {
		t.Errorf("attachment filename = %q, want %q", att.Filename, "photo.png")
	}

	// The critical check: decoded attachment data must match the original binary.
	if string(att.Data) != wantData {
		t.Errorf("attachment data mismatch:\n  got  (%d bytes) = %x\n  want (%d bytes) = %x",
			len(att.Data), att.Data, len(wantData), []byte(wantData))
	}
}

// TestParseBase64AttachmentNotRawBase64 ensures that the attachment data is
// NOT the raw base64 text (the old buggy behavior).
func TestParseBase64AttachmentNotRawBase64(t *testing.T) {
	email := testEmails[6]
	mboxData := generateMbox([]testEmail{email})
	messages, err := ParseReader(strings.NewReader(mboxData))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}

	att := messages[0].Attachments[0]
	// If the parser forgot to decode base64, the data would contain only
	// ASCII base64 characters. Our test data contains non-ASCII bytes (\x89,
	// \xff, \x00), so the decoded data should NOT be valid base64 text.
	encoded := base64.StdEncoding.EncodeToString([]byte(email.Attachments[0].Data))
	if string(att.Data) == encoded {
		t.Error("attachment data is still raw base64 — Content-Transfer-Encoding was not decoded")
	}
}

// TestParseUnEncodedAttachmentStillWorks verifies that attachments without
// Content-Transfer-Encoding (plain text) are not broken by the decoder.
func TestParseUnEncodedAttachmentStillWorks(t *testing.T) {
	// Email 2 has a plain-text attachment with no encoding.
	email := testEmails[2]
	mboxData := generateMbox([]testEmail{email})
	messages, err := ParseReader(strings.NewReader(mboxData))
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}

	att := messages[0].Attachments[0]
	// Trim whitespace since the parser may include trailing newlines.
	got := strings.TrimRight(string(att.Data), "\n\r ")
	want := strings.TrimRight(email.Attachments[0].Data, "\n\r ")
	if got != want {
		t.Errorf("unencoded attachment data:\n  got  = %q\n  want = %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// FilenameBase tests
// ---------------------------------------------------------------------------

// TestFilenameBaseNormal checks the basic format: "<date> <subject>".
func TestFilenameBaseNormal(t *testing.T) {
	msg := &Message{
		Date:    time.Date(2025, 3, 29, 10, 0, 0, 0, time.UTC),
		Subject: "Hello from umbox!",
	}
	got := msg.FilenameBase(DefaultMaxSubjectLen)
	want := "2025-03-29 Hello from umbox!"
	if got != want {
		t.Errorf("FilenameBase() = %q, want %q", got, want)
	}
}

// TestFilenameBaseUnsafeChars verifies that filesystem-unsafe characters
// are replaced with underscores.
func TestFilenameBaseUnsafeChars(t *testing.T) {
	msg := &Message{
		Date:    time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		Subject: "Re: Q1/Q2 report <draft> — final?",
	}
	got := msg.FilenameBase(DefaultMaxSubjectLen)
	// / < > ? should all become underscores.
	if strings.ContainsAny(got, "/\\:*?\"<>|") {
		t.Errorf("FilenameBase() contains unsafe chars: %q", got)
	}
	// The date prefix should still be there.
	if !strings.HasPrefix(got, "2025-01-15 ") {
		t.Errorf("FilenameBase() missing date prefix: %q", got)
	}
}

// TestFilenameBaseEmpty verifies the fallback when there's no subject.
func TestFilenameBaseEmpty(t *testing.T) {
	msg := &Message{
		Date:    time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		Subject: "",
	}
	got := msg.FilenameBase(DefaultMaxSubjectLen)
	if got != "2025-06-01 (no subject)" {
		t.Errorf("FilenameBase() = %q, want %q", got, "2025-06-01 (no subject)")
	}
}

// TestFilenameBaseLongSubject verifies that very long subjects are truncated.
func TestFilenameBaseLongSubject(t *testing.T) {
	msg := &Message{
		Date:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Subject: strings.Repeat("A", 200),
	}
	got := msg.FilenameBase(DefaultMaxSubjectLen)
	// "2006-01-02 " is 11 runes, plus DefaultMaxSubjectLen (50) = 61 runes max.
	if len([]rune(got)) > 61 {
		t.Errorf("FilenameBase() too long (%d runes): %q", len([]rune(got)), got)
	}
}

// TestFilenameBaseLongUnicodeSubject verifies that truncation works correctly
// with multi-byte UTF-8 characters. Slicing by bytes (not runes) would split
// a character like "ü" mid-sequence, producing an illegal byte sequence that
// the filesystem rejects.
func TestFilenameBaseLongUnicodeSubject(t *testing.T) {
	// Build a 200-rune subject using multi-byte characters (ü = 2 bytes each).
	msg := &Message{
		Date:    time.Date(2020, 3, 18, 0, 0, 0, 0, time.UTC),
		Subject: strings.Repeat("ü", 200),
	}
	got := msg.FilenameBase(DefaultMaxSubjectLen)

	// Must be valid UTF-8 — no broken runes.
	for i, r := range got {
		if r == '\uFFFD' {
			t.Errorf("FilenameBase() contains replacement char at byte %d (broken UTF-8): %q", i, got)
			break
		}
	}

	// Must be truncated to at most 61 runes (11 date prefix + 50 subject).
	if len([]rune(got)) > 61 {
		t.Errorf("FilenameBase() too long (%d runes): %q", len([]rune(got)), got)
	}
}

// TestFilenameBaseCustomMaxLen verifies that a custom max subject length works.
func TestFilenameBaseCustomMaxLen(t *testing.T) {
	msg := &Message{
		Date:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Subject: strings.Repeat("X", 200),
	}

	// With max 20, total should be at most 31 runes (11 date + 20 subject).
	got := msg.FilenameBase(20)
	if len([]rune(got)) > 31 {
		t.Errorf("FilenameBase(20) too long (%d runes): %q", len([]rune(got)), got)
	}

	// With max 0 (unlimited), subject should not be truncated.
	got = msg.FilenameBase(0)
	// 11 date prefix + 200 subject = 211 runes.
	if len([]rune(got)) != 211 {
		t.Errorf("FilenameBase(0) = %d runes, want 211", len([]rune(got)))
	}
}
