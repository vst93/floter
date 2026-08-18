package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestDescribeIsOneJSONDocument(t *testing.T) {
	var output bytes.Buffer
	if err := protocol("describe", bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(output.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if value["protocolVersion"] != "1.0" {
		t.Fatalf("unexpected protocol version: %v", value["protocolVersion"])
	}
}

func TestCompleteAcceptsProtocolRequest(t *testing.T) {
	input := bytes.NewBufferString(`{"command":"search","args":[],"cwd":"/tmp"}`)
	var output bytes.Buffer
	if err := protocol("complete", input, &output); err != nil {
		t.Fatal(err)
	}
	var value struct {
		Completions []map[string]any `json:"completions"`
	}
	if err := json.Unmarshal(output.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if len(value.Completions) != 1 || value.Completions[0]["label"] != "--type" {
		t.Fatalf("unexpected completions: %#v", value.Completions)
	}
}
