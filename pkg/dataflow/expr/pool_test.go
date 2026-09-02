package expr

import (
	"context"
	"testing"
	"time"
)

func TestEvalBoolTrueFalse(t *testing.T) {
	p := NewPool(2)
	ok, err := p.EvalBool(context.Background(), "$json.n > 3", map[string]any{"$json": map[string]any{"n": 5}})
	if err != nil || !ok {
		t.Fatalf("expected true, got ok=%v err=%v", ok, err)
	}
	ok, err = p.EvalBool(context.Background(), "$json.n > 3", map[string]any{"$json": map[string]any{"n": 1}})
	if err != nil || ok {
		t.Fatalf("expected false, got ok=%v err=%v", ok, err)
	}
}

func TestEvalBoolRejectsNonBoolean(t *testing.T) {
	p := NewPool(2)
	if _, err := p.EvalBool(context.Background(), "1 + 1", nil); err == nil {
		t.Fatal("expected error for non-boolean result")
	}
}

func TestCompileCacheReused(t *testing.T) {
	p := NewPool(2)
	src := "$json.n > 0"
	if _, err := p.Eval(context.Background(), src, map[string]any{"$json": map[string]any{"n": 1}}); err != nil {
		t.Fatalf("eval: %v", err)
	}
	p.mu.RLock()
	_, cached := p.programs[src]
	p.mu.RUnlock()
	if !cached {
		t.Fatal("expected compiled program to be cached")
	}
}

func TestRuntimeReusedAcrossEvaluations(t *testing.T) {
	p := NewPool(1)
	for i := 0; i < 5; i++ {
		ok, err := p.EvalBool(context.Background(), "true", nil)
		if err != nil || !ok {
			t.Fatalf("eval %d: ok=%v err=%v", i, ok, err)
		}
	}
	if len(p.runtimes) != 1 {
		t.Fatalf("expected the single runtime to be checked back in, pool size=%d", len(p.runtimes))
	}
}

func TestEvalTimesOutOnInfiniteLoop(t *testing.T) {
	p := NewPool(1)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := p.Eval(ctx, "while(true) {}", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
}
