# Default recipe — show available recipes when you just type "just"
default:
    @just --list

# Build the umbox binary
build:
    go build -o umbox .

# Run go mod tidy to sync dependencies
tidy:
    go mod tidy

# Format all Go source files (Go has an official formatter — always use it)
fmt:
    go fmt ./...

# Run all tests (mbox parser + formatter)
test:
    go test ./...

# Run tests with verbose output (shows each test name)
test-v:
    go test ./... -v

# Build and extract sample emails as .eml files
sample-extract: build
    ./umbox extract testdata/test.mbox -o /tmp/umbox-sample-eml
    @echo "\nOutput:"
    @ls -l /tmp/umbox-sample-eml

# Build and convert sample emails to markdown
sample-markdown: build
    ./umbox convert testdata/test.mbox -f markdown -o /tmp/umbox-sample-md
    @echo "\nOutput:"
    @ls -l /tmp/umbox-sample-md

# Build and convert sample emails to plain text
sample-plaintext: build
    ./umbox convert testdata/test.mbox -f plaintext -o /tmp/umbox-sample-txt
    @echo "\nOutput:"
    @ls -l /tmp/umbox-sample-txt

# Launch the interactive TUI browser with sample data
browse: build
    ./umbox browse testdata/test.mbox

# Run all sample recipes
sample-all: sample-extract sample-markdown sample-plaintext

# Clean build artifacts and sample output
clean:
    rm -f umbox
    rm -rf /tmp/umbox-sample-*
