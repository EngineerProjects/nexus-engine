package database

import "testing"

func TestMongoDBValidateParameters(t *testing.T) {
	n := NewMongoDB()
	if err := n.ValidateParameters(map[string]any{}); err == nil {
		t.Fatal("expected error for missing uriSecretRef")
	}
	if err := n.ValidateParameters(map[string]any{"uriSecretRef": "x", "database": "d", "collection": "c", "operation": "bogus"}); err == nil {
		t.Fatal("expected error for invalid operation")
	}
	if err := n.ValidateParameters(map[string]any{"uriSecretRef": "x", "database": "d", "collection": "c", "operation": "find"}); err != nil {
		t.Fatalf("expected valid params, got %v", err)
	}
}

func TestToBsonM(t *testing.T) {
	if len(toBsonM(nil)) != 0 {
		t.Fatal("expected empty bson.M for nil input")
	}
	m := toBsonM(map[string]any{"a": 1})
	if m["a"] != 1 {
		t.Fatalf("unexpected conversion: %#v", m)
	}
}
