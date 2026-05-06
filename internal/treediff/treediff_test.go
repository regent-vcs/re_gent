package treediff

import (
	"testing"
)

func TestIsBinaryContent(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{"empty", []byte{}, false},
		{"text", []byte("hello world"), false},
		{"binary with null", []byte{0x00, 0x01, 0x02}, true},
		{"text with newlines", []byte("line1\nline2\n"), false},
		{"binary in middle", []byte("text\x00more"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBinaryContent(tt.content)
			if result != tt.expected {
				t.Errorf("isBinaryContent(%q) = %v, want %v", tt.content, result, tt.expected)
			}
		})
	}
}

func TestFormatFileStat(t *testing.T) {
	// This is tested via the CLI package since FileDiff is defined there
	// Just verify the treediff package compiles
}

func TestComputeLineStats(t *testing.T) {
	// Integration test - would need a real store
	// Just verify compilation
}
