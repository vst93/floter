package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

//go:embed provider-description.json
var description []byte

type completeRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	CWD     string   `json:"cwd"`
}

func protocol(operation string, input io.Reader, output io.Writer) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	switch operation {
	case "describe":
		var value any
		if err := json.Unmarshal(description, &value); err != nil {
			return fmt.Errorf("invalid embedded description: %w", err)
		}
		return encoder.Encode(value)
	case "complete":
		var request completeRequest
		if err := json.NewDecoder(input).Decode(&request); err != nil {
			return fmt.Errorf("invalid completion request: %w", err)
		}
		items := []any{}
		if request.Command == "search" {
			items = append(items, map[string]any{
				"label": "--type",
				"kind":  "flag",
				"detail": fmt.Sprintf("Filter resource type (%d argument(s), cwd %s)", len(request.Args), request.CWD),
			})
		}
		return encoder.Encode(map[string]any{"completions": items})
	case "diagnose":
		return encoder.Encode(map[string]any{
			"status": "ok",
			"checks": []any{map[string]any{
				"id": "runtime", "status": "ok", "message": "Provider is ready",
			}},
		})
	case "config":
		return encoder.Encode(map[string]any{
			"configuration": map[string]any{
				"configVersion": 1,
				"owner":       "host",
				"schema": []any{map[string]any{
					"key": "endpoint", "type": "text", "label": "API endpoint",
					"default": "https://api.example.com",
				}},
			},
		})
	default:
		return fmt.Errorf("unsupported Floter operation: %s", operation)
	}
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] != "--floter" {
		fmt.Fprintln(os.Stderr, "run the wrapped tool here")
		return
	}
	if len(args) != 4 || args[2] != "--protocol" || args[3] != "1" {
		fmt.Fprintln(os.Stderr, "usage: --floter <operation> --protocol 1")
		os.Exit(2)
	}
	if err := protocol(args[1], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
