# umbox

A CLI tool for extracting and converting emails from mbox archive files.

## What it does

- **Browse** emails interactively in a terminal UI (search, select, export)
- **Extract** emails from `.mbox` files as individual `.eml` files (standard email format)
- **Convert** emails to human-readable formats: plain text (`.txt`) or Markdown (`.md`)
- Saves **attachments** alongside converted emails

## Prerequisites

### Install Go

umbox requires [Go](https://go.dev/) 1.21 or later.

**macOS (Homebrew):**
```bash
brew install go
```

**macOS/Linux (official installer):**
Download from https://go.dev/dl/ and follow the instructions.

**Verify installation:**
```bash
go version
# Should print something like: go version go1.26.1 darwin/arm64
```

### Go concepts for newcomers

If you've never used Go before, here are the key things to know:

- **`go.mod`** — Like `package.json` (Node) or `pyproject.toml` (Python). Defines the module name and dependencies.
- **`go build`** — Compiles your code into a single binary. No runtime needed!
- **`go run .`** — Compiles and runs in one step (useful during development).
- **`go mod tidy`** — Adds missing dependencies and removes unused ones (like `npm install`).
- **Packages** — Each folder is a "package". Files in the same folder share the same package namespace.
- **Exported vs unexported** — Names starting with an uppercase letter (like `Parse`) are public. Lowercase names (like `parseMessage`) are private to the package.

## Setup

```bash
# Clone the repository
git clone https://github.com/jneuendorf/umbox.git
cd umbox

# Download dependencies (cobra CLI framework)
go mod tidy

# Build the binary
go build -o umbox .

# (Optional) Install globally — puts the binary in your $GOPATH/bin
go install .
```

## Usage

### Browse emails interactively (TUI)

```bash
./umbox browse inbox.mbox
```

This opens a terminal UI with:
- **Left pane**: Scrollable email list with selection checkboxes
- **Right pane**: Preview of the highlighted email
- **Search**: Press `/` to filter by sender, subject, or body text
- **Export**: Select emails with `space`, then press `e` to export

Key bindings:
| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate email list |
| `tab` | Switch focus between list and preview |
| `space` | Toggle select current email |
| `a` | Select/deselect all |
| `/` | Search/filter |
| `e` | Export selected emails |
| `q` | Quit |

### Extract emails as .eml files

```bash
# Extract all emails to ./output/ (default)
./umbox extract inbox.mbox

# Extract to a specific directory
./umbox extract inbox.mbox -o ./my-emails
```

Each email becomes a numbered `.eml` file (e.g., `001.eml`, `002.eml`). EML files can be opened by most email clients (Thunderbird, Outlook, Apple Mail).

### Convert emails to readable formats

```bash
# Convert to plain text (default format)
./umbox convert inbox.mbox -o ./readable

# Convert to Markdown
./umbox convert inbox.mbox -f markdown -o ./readable

# Convert to plain text (explicit)
./umbox convert inbox.mbox -f plaintext -o ./readable
```

Attachments are saved in numbered subfolders alongside each email:
```
readable/
├── 001.md
├── 001_attachments/
│   ├── report.pdf
│   └── photo.jpg
├── 002.md
└── 003.md
```

### Help

```bash
./umbox --help
./umbox extract --help
./umbox browse --help
./umbox convert --help
```

## Project Structure

```
umbox/
├── main.go              # Entry point — just calls cmd.Execute()
├── cmd/                 # CLI commands (thin wrappers around core logic)
│   ├── root.go          # Base command + help text
│   ├── browse.go        # "browse" subcommand (launches TUI)
│   ├── extract.go       # "extract" subcommand
│   └── convert.go       # "convert" subcommand
├── tui/                 # Interactive terminal UI (Bubble Tea)
│   ├── tui.go           # Main model — Init/Update/View + Run()
│   ├── keymap.go        # Key binding definitions
│   └── styles.go        # lipgloss color/layout styles
├── mbox/                # Core library — parsing mbox files
│   ├── message.go       # Message and Attachment data types
│   └── parser.go        # Mbox file parser
└── formatter/           # Output format system (extensible)
    ├── formatter.go     # Formatter interface
    ├── registry.go      # Format registry (lookup by name)
    ├── plaintext.go     # Plain text output
    └── markdown.go      # Markdown output
```

The architecture is modular by design:
- **`mbox/`** handles all parsing — no I/O decisions, no formatting
- **`formatter/`** handles all output formatting — pluggable via an interface
- **`tui/`** imports `mbox` and `formatter` directly — no logic duplication
- **`cmd/`** is just glue code that wires everything together

## Adding a New Output Format

The formatter system is designed to be extended. To add a new format (e.g., HTML):

1. Create a new file `formatter/html.go`
2. Implement the `Formatter` interface:

```go
package formatter

import (
    "fmt"
    "io"

    "github.com/jneuendorf/umbox/mbox"
)

// init registers this formatter automatically when the package is imported.
func init() {
    Register(&HTMLFormatter{})
}

type HTMLFormatter struct{}

func (f *HTMLFormatter) Name() string      { return "html" }
func (f *HTMLFormatter) Extension() string { return ".html" }

func (f *HTMLFormatter) Format(msg *mbox.Message, w io.Writer) error {
    fmt.Fprintf(w, "<html><body>")
    fmt.Fprintf(w, "<h1>%s</h1>", msg.Subject)
    // ... your HTML formatting logic here ...
    fmt.Fprintf(w, "</body></html>")
    return nil
}
```

That's it! The `init()` function registers the formatter automatically, and it becomes available via `--format html` on the CLI.

## Where to get mbox files

- **Gmail**: Google Takeout → select "Mail" → downloads as `.mbox`
- **Thunderbird**: Right-click a folder → "ImportExportTools NG" addon → Export as mbox
- **Apple Mail**: Mailbox → Export Mailbox

## Development

```bash
# Run without building (useful during development)
go run . extract inbox.mbox -o ./test-output

# Build
go build -o umbox .

# Run tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Format code (Go has an official formatter — always use it)
go fmt ./...
```

## Roadmap

- [x] Extract emails as .eml files
- [x] Convert to plain text
- [x] Convert to Markdown
- [x] TUI for browsing and selectively exporting emails
- [ ] HTML output format
- [ ] Additional search filters (date range, attachment presence)
