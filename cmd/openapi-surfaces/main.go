package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/korwlik23/go-api-starter/internal/openapitools"
)

func main() {
	input := flag.String("input", "openapi/dist/root.json", "bundled root OpenAPI JSON")
	outputDir := flag.String("output-dir", "openapi/dist", "surface source output directory")
	flag.Parse()
	if err := run(*input, *outputDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input, outputDir string) error {
	contents, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("read root OpenAPI bundle: %w", err)
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		return fmt.Errorf("decode root OpenAPI bundle: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("create OpenAPI output directory: %w", err)
	}
	for _, surface := range []string{"admin", "public", "site-server"} {
		filtered, err := openapitools.FilterDocument(document, surface)
		if err != nil {
			return fmt.Errorf("filter %s surface: %w", surface, err)
		}
		output, err := json.MarshalIndent(filtered, "", "  ")
		if err != nil {
			return fmt.Errorf("encode %s surface: %w", surface, err)
		}
		output = append(output, '\n')
		path := filepath.Join(outputDir, surface+".source.json")
		if err := os.WriteFile(path, output, 0o644); err != nil {
			return fmt.Errorf("write %s surface: %w", surface, err)
		}
	}
	return nil
}
