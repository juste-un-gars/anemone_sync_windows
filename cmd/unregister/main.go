//go:build windows

package main

import (
	"fmt"
	"os"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cloudfiles"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: unregister <path>")
		os.Exit(1)
	}

	path := os.Args[1]
	fmt.Printf("Unregistering sync root: %s\n", path)

	if err := cloudfiles.UnregisterSyncRoot(path); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Success!")
}
