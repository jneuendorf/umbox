package mbox

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"strings"
)

// Parse reads an mbox file from disk and returns all messages found in it.
// The mbox format is simple: each email starts with a line beginning with "From "
// (note the space after "From"). Everything until the next "From " line is one email.
//
// This function is the main entry point for reading mbox files. Both the CLI
// commands and a future TUI would call this function.
func Parse(filepath string) ([]*Message, error) {
	// os.Open opens a file for reading. It returns a file handle and an error.
	// In Go, functions often return (result, error) — you must always check the error.
	file, err := os.Open(filepath)
	if err != nil {
		// fmt.Errorf creates a formatted error message. The %w "wraps" the original
		// error so callers can inspect it if needed.
		return nil, fmt.Errorf("failed to open mbox file: %w", err)
	}
	// "defer" schedules a function call to run when the surrounding function returns.
	// This ensures the file gets closed even if we return early due to an error.
	defer file.Close()

	return ParseReader(file)
}

// ParseReader reads mbox-formatted data from any io.Reader (file, network, etc.)
// and returns the parsed messages. This is separated from Parse so it can be
// tested easily and used with data sources other than files.
func ParseReader(r io.Reader) ([]*Message, error) {
	// A Scanner reads input line by line — similar to a buffered reader in other languages.
	scanner := bufio.NewScanner(r)

	// Increase the default buffer size to handle very long lines (e.g., base64 attachments).
	// The default is 64KB; we set it to 1MB.
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	// messages will hold all parsed emails. In Go, "var" declares a variable.
	// A nil slice works fine — you can append to it without initializing it.
	var messages []*Message

	// currentRaw accumulates the raw bytes of the email we're currently reading.
	var currentRaw bytes.Buffer

	// inMessage tracks whether we've started reading an email.
	inMessage := false

	// Read the file line by line.
	for scanner.Scan() {
		line := scanner.Text() // Get the current line as a string.

		// Check if this line starts a new email. Mbox "From " lines mark boundaries.
		if strings.HasPrefix(line, "From ") {
			// If we were already reading an email, parse and save it.
			if inMessage && currentRaw.Len() > 0 {
				msg, err := parseMessage(currentRaw.Bytes())
				if err != nil {
					// We log the error but keep going — one bad email shouldn't
					// prevent us from parsing the rest of the file.
					fmt.Fprintf(os.Stderr, "warning: skipping malformed message: %v\n", err)
				} else {
					messages = append(messages, msg)
				}
				currentRaw.Reset() // Clear the buffer for the next email.
			}
			inMessage = true
			continue // Skip the "From " line itself — it's not part of the email.
		}

		// If we're inside an email, accumulate its content.
		if inMessage {
			// Write the line plus a newline back into our buffer.
			currentRaw.WriteString(line)
			currentRaw.WriteByte('\n')
		}
	}

	// Don't forget the last email in the file (there's no trailing "From " line).
	if inMessage && currentRaw.Len() > 0 {
		msg, err := parseMessage(currentRaw.Bytes())
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping malformed message: %v\n", err)
		} else {
			messages = append(messages, msg)
		}
	}

	// Check if the scanner itself hit an error (e.g., disk read failure).
	if err := scanner.Err(); err != nil {
		return messages, fmt.Errorf("error reading mbox data: %w", err)
	}

	return messages, nil
}

// parseMessage takes the raw bytes of a single email and parses it into our
// Message struct. This uses Go's standard library "net/mail" package which
// understands the RFC 5322 email format.
func parseMessage(raw []byte) (*Message, error) {
	// mail.ReadMessage parses email headers and gives us access to the body.
	// bytes.NewReader wraps our byte slice so it satisfies the io.Reader interface.
	mailMsg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("failed to parse email: %w", err)
	}

	// Build our Message struct from the parsed email.
	msg := &Message{
		// The "&" creates a pointer to the struct. In Go, you almost always work
		// with pointers to structs to avoid copying large data around.
		MessageID: mailMsg.Header.Get("Message-ID"),
		From:      mailMsg.Header.Get("From"),
		Subject:   mailMsg.Header.Get("Subject"),
		Headers:   map[string][]string(mailMsg.Header),
		RawBytes:  raw,
	}

	// Parse the "To" header which may contain multiple comma-separated addresses.
	if to := mailMsg.Header.Get("To"); to != "" {
		// Split on commas and trim whitespace from each address.
		for _, addr := range strings.Split(to, ",") {
			msg.To = append(msg.To, strings.TrimSpace(addr))
		}
	}

	// Parse the date. mail.Header.Date() understands various email date formats.
	if date, err := mailMsg.Header.Date(); err == nil {
		msg.Date = date
	}

	// Parse the body. Emails can be simple (just text) or complex (multipart with
	// attachments). The Content-Type header tells us which.
	contentType := mailMsg.Header.Get("Content-Type")
	if contentType == "" {
		// No Content-Type means plain text by convention.
		contentType = "text/plain"
	}

	// mime.ParseMediaType splits "text/html; charset=utf-8" into the type
	// ("text/html") and parameters ({"charset": "utf-8"}).
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// If we can't parse the content type, treat the whole body as plain text.
		body, _ := io.ReadAll(mailMsg.Body)
		msg.TextBody = string(body)
		return msg, nil
	}

	// Check if this is a multipart message (has attachments or HTML + text versions).
	if strings.HasPrefix(mediaType, "multipart/") {
		err = parseMultipart(mailMsg.Body, params["boundary"], msg)
		if err != nil {
			// If multipart parsing fails, fall back to reading raw body.
			body, _ := io.ReadAll(mailMsg.Body)
			msg.TextBody = string(body)
		}
	} else {
		// Simple single-part message — just read the body.
		body, err := io.ReadAll(mailMsg.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read message body: %w", err)
		}
		if strings.HasPrefix(mediaType, "text/html") {
			msg.HTMLBody = string(body)
		} else {
			msg.TextBody = string(body)
		}
	}

	return msg, nil
}

// parseMultipart handles multipart MIME messages. These are emails that contain
// multiple "parts" — for example, a text version AND an HTML version, plus
// attachments. The parts are separated by a "boundary" string.
func parseMultipart(body io.Reader, boundary string, msg *Message) error {
	// multipart.NewReader creates a reader that can iterate over MIME parts.
	reader := multipart.NewReader(body, boundary)

	// Loop through each part of the multipart message.
	for {
		// NextPart returns the next MIME part, or io.EOF when there are no more.
		part, err := reader.NextPart()
		if err == io.EOF {
			break // No more parts — we're done.
		}
		if err != nil {
			return fmt.Errorf("failed to read multipart section: %w", err)
		}

		// Read this part's content into memory.
		partData, err := io.ReadAll(part)
		if err != nil {
			continue // Skip parts we can't read.
		}

		// Determine what kind of part this is.
		partContentType := part.Header.Get("Content-Type")
		partMediaType, partParams, _ := mime.ParseMediaType(partContentType)
		disposition := part.Header.Get("Content-Disposition")

		// If it has a filename or is explicitly an attachment, treat it as one.
		if part.FileName() != "" || strings.HasPrefix(disposition, "attachment") {
			msg.Attachments = append(msg.Attachments, Attachment{
				Filename:    part.FileName(),
				ContentType: partContentType,
				Data:        partData,
			})
			continue
		}

		// Check if this part is itself multipart (nested multipart messages are common).
		if strings.HasPrefix(partMediaType, "multipart/") {
			// Recursively parse nested multipart content.
			nestedReader := bytes.NewReader(partData)
			err = parseMultipart(nestedReader, partParams["boundary"], msg)
			if err != nil {
				// If nested parsing fails, just skip it.
				continue
			}
		} else if strings.HasPrefix(partMediaType, "text/html") {
			msg.HTMLBody = string(partData)
		} else if strings.HasPrefix(partMediaType, "text/plain") || partMediaType == "" {
			msg.TextBody = string(partData)
		}
	}

	return nil
}
