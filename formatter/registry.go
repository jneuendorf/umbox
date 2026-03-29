package formatter

import (
	"fmt"
	"sort"
)

// registry is a package-level variable that maps formatter names to their
// implementations. It's private (lowercase) so outside code can't mess with
// it directly — they must use the Register/Get/List functions below.
//
// In Go, package-level variables are initialized when the program starts.
var registry = make(map[string]Formatter)

// Register adds a formatter to the registry. Call this in your formatter's
// init() function to make it available automatically.
//
// init() is a special Go function that runs automatically when a package is
// imported — you never call it yourself. It's perfect for self-registration.
//
// Example usage in a new formatter file:
//
//	func init() {
//	    Register(&MyFormatter{})
//	}
func Register(f Formatter) {
	registry[f.Name()] = f
}

// Get retrieves a formatter by name. Returns an error if the name isn't found.
// The second return value follows Go convention: (result, error).
func Get(name string) (Formatter, error) {
	f, ok := registry[name]
	// "ok" is a boolean that tells us if the key was found in the map.
	// This is Go's idiomatic way to check map membership.
	if !ok {
		return nil, fmt.Errorf("unknown format %q, available: %v", name, List())
	}
	return f, nil
}

// List returns the names of all registered formatters, sorted alphabetically.
// This is useful for help text and error messages.
func List() []string {
	// make() creates a slice with a specific length. We know exactly how many
	// names we need, so we pre-allocate the right size.
	names := make([]string, 0, len(registry))

	// "range" iterates over maps, slices, and other collections.
	// For maps, it gives you (key, value) on each iteration.
	// The underscore "_" means we don't need the value, just the key.
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
