package main

import (
	"fmt"
	"log"
	"os"
)

// These variables will be set at build time by goreleaser
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("ai-code-template-go %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built at: %s\n", date)
		fmt.Printf("  built by: %s\n", builtBy)
		return
	}

	log.Println("Hello from ai-code-template-go!")
	log.Printf("This is version %s (commit %s)", version, commit)
}
