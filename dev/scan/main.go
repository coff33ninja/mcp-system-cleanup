package main

import (
	"fmt"

	"systemcleanup/mcp/internal/cleaner"
	"systemcleanup/mcp/internal/langmgr"
)

func main() {
	opts := cleaner.Options{MinSizeMB: 10}
	targets := cleaner.Scan(opts)
	fmt.Printf("SCAN: %d targets >=10MB, total %.0f MB\n", len(targets), func() float64 {
		var t float64
		for _, x := range targets {
			t += x.SizeMB
		}
		return t
	}())
	for _, t := range targets {
		fmt.Printf("  [%2d] %-55s %8.1f MB\n", t.Category, t.Label, t.SizeMB)
	}

	det := langmgr.Detect()
	fmt.Println("\nDETECT:")
	for _, d := range det {
		mark := "-"
		if d.Present {
			mark = "Y"
		}
		fmt.Printf("  [%s] %-18s %s\n", mark, d.Name, d.Version)
	}
}
