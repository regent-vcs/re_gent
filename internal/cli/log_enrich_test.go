package cli

import (
	"encoding/json"
	"testing"
)

func TestRawJSONForOutput_PreservesValidJSON(t *testing.T) {
	raw := rawJSONForOutput([]byte(`{"ok":true}`))
	if string(raw) != `{"ok":true}` {
		t.Fatalf("rawJSONForOutput() = %s", raw)
	}
}

func TestRawJSONForOutput_WrapsLegacyPlainText(t *testing.T) {
	raw := rawJSONForOutput([]byte(`ok`))
	if string(raw) != `"ok"` {
		t.Fatalf("rawJSONForOutput() = %s, want JSON string", raw)
	}

	var decoded string
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("wrapped output should be valid JSON: %v", err)
	}
	if decoded != "ok" {
		t.Fatalf("decoded output = %q", decoded)
	}
}
