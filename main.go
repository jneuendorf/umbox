// main.go is the entry point for the umbox CLI tool.
// In Go, every executable program must have a "main" package with a "main" function.
// This file is intentionally minimal — all the real logic lives in the cmd/ package.
package main

// "import" brings in code from other packages, similar to Python's import or JS's require.
// Here we import our own "cmd" package which defines the CLI commands.
import "github.com/jneuendorf/umbox/cmd"

// main is the function Go calls when you run the program.
func main() {
	// Execute sets up and runs the CLI. If something goes wrong,
	// it prints an error and exits — we don't need to handle errors here.
	cmd.Execute()
}
