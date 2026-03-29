package formatter

// Tests for the formatter package: registry, plaintext output, markdown output.
// Run with: go test ./formatter/ -v

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jneuendorf/umbox/mbox"
)

// ---------------------------------------------------------------------------
// Test helpers — shared message fixtures
// ---------------------------------------------------------------------------

// newTestMessage creates a Message with the given fields, used across tests.
// Using a helper keeps the test cases focused on what they're actually testing.
func newTestMessage(from, subject, body string, to []string) *mbox.Message {
	return &mbox.Message{
		From:    from,
		To:      to,
		Subject: subject,
		Date:    time.Date(2025, 3, 29, 10, 0, 0, 0, time.UTC),
		TextBody: body,
	}
}

// newTestMessageWithAttachment creates a Message that has an attachment.
func newTestMessageWithAttachment() *mbox.Message {
	return &mbox.Message{
		From:    "Dave <dave@example.com>",
		To:      []string{"team@example.com"},
		Subject: "Report with attachment",
		Date:    time.Date(2025, 3, 29, 12, 0, 0, 0, time.UTC),
		TextBody: "See attached.",
		Attachments: []mbox.Attachment{
			{
				Filename:    "report.pdf",
				ContentType: "application/pdf",
				Data:        []byte("fake-pdf-data"),
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Registry tests
// ---------------------------------------------------------------------------

// TestRegistryList verifies that both built-in formatters are registered.
func TestRegistryList(t *testing.T) {
	names := List()

	// We expect "markdown", "plaintext", and "raw" (registered via init()).
	expected := map[string]bool{"markdown": false, "plaintext": false, "raw": false}
	for _, name := range names {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("formatter %q not found in registry, got: %v", name, names)
		}
	}
}

// TestRegistryGet checks that Get returns the correct formatter and errors.
func TestRegistryGet(t *testing.T) {
	// Valid names should return a formatter.
	for _, name := range []string{"plaintext", "markdown", "raw"} {
		f, err := Get(name)
		if err != nil {
			t.Errorf("Get(%q) returned error: %v", name, err)
			continue
		}
		if f.Name() != name {
			t.Errorf("Get(%q).Name() = %q", name, f.Name())
		}
	}

	// Invalid name should return an error.
	_, err := Get("nonexistent")
	if err == nil {
		t.Error("Get(\"nonexistent\") should return an error, got nil")
	}
}

// TestRegistryListSorted verifies that List returns names in alphabetical order.
func TestRegistryListSorted(t *testing.T) {
	names := List()
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("List() not sorted: %v", names)
			break
		}
	}
}

// ---------------------------------------------------------------------------
// Plaintext formatter tests
// ---------------------------------------------------------------------------

// TestPlaintextBasic checks that the plaintext formatter includes all expected fields.
func TestPlaintextBasic(t *testing.T) {
	msg := newTestMessage(
		"Alice <alice@example.com>",
		"Hello from umbox!",
		"Hey Bob, this is a test.",
		[]string{"Bob <bob@example.com>"},
	)

	f, _ := Get("plaintext")
	var buf bytes.Buffer
	if err := f.Format(msg, &buf); err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	output := buf.String()

	// Verify all key fields appear in the output.
	checks := []struct {
		label string
		want  string
	}{
		{"From header", "Alice <alice@example.com>"},
		{"To header", "Bob <bob@example.com>"},
		{"Subject header", "Hello from umbox!"},
		{"Body content", "Hey Bob, this is a test."},
		{"Date", "2025"},
	}
	for _, check := range checks {
		if !strings.Contains(output, check.want) {
			t.Errorf("plaintext output missing %s: %q not found in:\n%s", check.label, check.want, output)
		}
	}
}

// TestPlaintextExtension verifies the file extension.
func TestPlaintextExtension(t *testing.T) {
	f, _ := Get("plaintext")
	if ext := f.Extension(); ext != ".txt" {
		t.Errorf("plaintext Extension() = %q, want %q", ext, ".txt")
	}
}

// TestPlaintextAttachments checks that attachments are listed in the output.
func TestPlaintextAttachments(t *testing.T) {
	msg := newTestMessageWithAttachment()
	f, _ := Get("plaintext")
	var buf bytes.Buffer
	if err := f.Format(msg, &buf); err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "report.pdf") {
		t.Error("plaintext output should list attachment filename")
	}
	if !strings.Contains(output, "application/pdf") {
		t.Error("plaintext output should list attachment content type")
	}
	if !strings.Contains(output, "Attachments (1)") {
		t.Error("plaintext output should show attachment count")
	}
}

// TestPlaintextNoAttachmentSection verifies that emails without attachments
// don't show an attachments section.
func TestPlaintextNoAttachmentSection(t *testing.T) {
	msg := newTestMessage("a@b.com", "Subject", "Body", []string{"c@d.com"})
	f, _ := Get("plaintext")
	var buf bytes.Buffer
	f.Format(msg, &buf)
	if strings.Contains(buf.String(), "Attachments") {
		t.Error("plaintext output should not show Attachments section for emails without attachments")
	}
}

// ---------------------------------------------------------------------------
// Markdown formatter tests
// ---------------------------------------------------------------------------

// TestMarkdownBasic checks that the markdown formatter produces valid markdown
// with all expected fields.
func TestMarkdownBasic(t *testing.T) {
	msg := newTestMessage(
		"Alice <alice@example.com>",
		"Hello from umbox!",
		"Hey Bob, this is a test.",
		[]string{"Bob <bob@example.com>"},
	)

	f, _ := Get("markdown")
	var buf bytes.Buffer
	if err := f.Format(msg, &buf); err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	output := buf.String()

	checks := []struct {
		label string
		want  string
	}{
		{"H1 heading with subject", "# Hello from umbox!"},
		{"From in table", "Alice <alice@example.com>"},
		{"To in table", "Bob <bob@example.com>"},
		{"Body content", "Hey Bob, this is a test."},
		{"Table header", "| Field"},
		{"Horizontal rule", "---"},
	}
	for _, check := range checks {
		if !strings.Contains(output, check.want) {
			t.Errorf("markdown output missing %s: %q not found in:\n%s", check.label, check.want, output)
		}
	}
}

// TestMarkdownExtension verifies the file extension.
func TestMarkdownExtension(t *testing.T) {
	f, _ := Get("markdown")
	if ext := f.Extension(); ext != ".md" {
		t.Errorf("markdown Extension() = %q, want %q", ext, ".md")
	}
}

// TestMarkdownAttachments checks that attachments appear in a markdown section.
func TestMarkdownAttachments(t *testing.T) {
	msg := newTestMessageWithAttachment()
	f, _ := Get("markdown")
	var buf bytes.Buffer
	f.Format(msg, &buf)
	output := buf.String()

	if !strings.Contains(output, "## Attachments (1)") {
		t.Error("markdown output should have attachments heading")
	}
	if !strings.Contains(output, "`report.pdf`") {
		t.Error("markdown output should show filename in backticks")
	}
}

// TestMarkdownNoAttachmentSection verifies clean output for emails without attachments.
func TestMarkdownNoAttachmentSection(t *testing.T) {
	msg := newTestMessage("a@b.com", "Subject", "Body", []string{"c@d.com"})
	f, _ := Get("markdown")
	var buf bytes.Buffer
	f.Format(msg, &buf)
	if strings.Contains(buf.String(), "## Attachments") {
		t.Error("markdown output should not show Attachments section for emails without attachments")
	}
}

// TestMarkdownMultipleRecipients checks comma-separated To rendering.
func TestMarkdownMultipleRecipients(t *testing.T) {
	msg := newTestMessage(
		"sender@example.com",
		"Test",
		"Body",
		[]string{"alice@example.com", "bob@example.com"},
	)
	f, _ := Get("markdown")
	var buf bytes.Buffer
	f.Format(msg, &buf)
	output := buf.String()

	if !strings.Contains(output, "alice@example.com, bob@example.com") {
		t.Errorf("expected comma-separated recipients, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// Raw formatter tests
// ---------------------------------------------------------------------------

// TestRawOutputMatchesRawBytes checks that the raw formatter writes the exact
// RawBytes content without any transformation.
func TestRawOutputMatchesRawBytes(t *testing.T) {
	rawContent := []byte("From: test@example.com\nSubject: Test\n\nHello world\n")
	msg := &mbox.Message{
		RawBytes: rawContent,
	}

	f, _ := Get("raw")
	var buf bytes.Buffer
	if err := f.Format(msg, &buf); err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !bytes.Equal(buf.Bytes(), rawContent) {
		t.Errorf("raw output differs from RawBytes:\n  got  = %q\n  want = %q", buf.Bytes(), rawContent)
	}
}

// TestRawExtension verifies the file extension.
func TestRawExtension(t *testing.T) {
	f, _ := Get("raw")
	if ext := f.Extension(); ext != ".eml" {
		t.Errorf("raw Extension() = %q, want %q", ext, ".eml")
	}
}

// TestRawName verifies the formatter name.
func TestRawName(t *testing.T) {
	f, _ := Get("raw")
	if name := f.Name(); name != "raw" {
		t.Errorf("raw Name() = %q, want %q", name, "raw")
	}
}
