// Command news drafts a site/news.json entry for the newest release in
// CHANGELOG.md. See docs/WRITING_FOR_PLAYERS.md.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "draft" {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/news draft   (from the repo root)")
		os.Exit(2)
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	data, err := os.ReadFile("site/news.json")
	if err != nil {
		return err
	}
	entries, err := parseNews(data)
	if err != nil {
		return fmt.Errorf("site/news.json: %w", err)
	}
	changelog, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		return err
	}
	entry, err := draft(string(changelog), entries)
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(entry, "  ", "  ")
	if err != nil {
		return err
	}
	fmt.Println("  " + string(out) + ",")
	return nil
}
