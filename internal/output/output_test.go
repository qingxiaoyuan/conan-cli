package output

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestJSONErrorIncludesActionAndExitCode(t *testing.T) {
	var out bytes.Buffer
	printer := New(true, &out, &bytes.Buffer{})
	printer.Error("install", codedError{code: 6})

	var response map[string]any
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["ok"] != false || response["action"] != "install" || response["exit_code"] != float64(6) {
		t.Fatalf("unexpected response: %#v", response)
	}
}

type codedError struct{ code int }

func (e codedError) Error() string { return "failed" }

func (e codedError) ExitCode() int { return e.code }
