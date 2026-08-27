package store

import (
	"context"
	"errors"
	"testing"

	"mural-conservation-gate/internal/domain"
)

func TestIdempotencyRejectsDifferentFingerprint(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err = s.SaveIdempotency(ctx, "key", "first", []byte(`{"ok":true}`), 200); err != nil {
		t.Fatal(err)
	}
	result, err := s.LookupIdempotency(ctx, "key", "first")
	if err != nil || !result.Found {
		t.Fatalf("应复用同载荷响应: %v", err)
	}
	_, err = s.LookupIdempotency(ctx, "key", "second")
	if !errors.Is(err, domain.ErrIdempotencyReuse) {
		t.Fatalf("应拒绝异载荷: %v", err)
	}
}
