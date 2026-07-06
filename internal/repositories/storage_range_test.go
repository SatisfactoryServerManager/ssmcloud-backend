package repositories

import (
	"fmt"
	"testing"
)

// rangeHeaderFor is the pure helper the implementation must expose so we can
// test Range formatting without hitting S3.
func TestRangeHeaderFor(t *testing.T) {
	if got := rangeHeaderFor(0); got != "" {
		t.Fatalf("offset 0 should produce empty range, got %q", got)
	}
	if got := rangeHeaderFor(1024); got != fmt.Sprintf("bytes=%d-", 1024) {
		t.Fatalf("unexpected range header: %q", got)
	}
}
