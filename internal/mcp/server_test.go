package mcp

import "testing"

func TestNew(t *testing.T) {
	if srv := New(); srv == nil {
		t.Fatal("New() returned nil server")
	}
}
