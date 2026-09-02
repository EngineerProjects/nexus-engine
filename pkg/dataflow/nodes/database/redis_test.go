package database

import "testing"

func TestRedisValidateParameters(t *testing.T) {
	n := NewRedis()
	if err := n.ValidateParameters(map[string]any{}); err == nil {
		t.Fatal("expected error for missing addrSecretRef")
	}
	if err := n.ValidateParameters(map[string]any{"addrSecretRef": "x", "operation": "bogus", "key": "k"}); err == nil {
		t.Fatal("expected error for invalid operation")
	}
	if err := n.ValidateParameters(map[string]any{"addrSecretRef": "x", "operation": "get", "key": "k"}); err != nil {
		t.Fatalf("expected valid params, got %v", err)
	}
}
