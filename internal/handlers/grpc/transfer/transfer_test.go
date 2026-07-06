package transfer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/config"
)

func TestStagedOffsetAndAppend(t *testing.T) {
	config.DataDir = t.TempDir() // ensure temp dir base is writable
	_ = os.MkdirAll(filepath.Join(config.DataDir, "temp"), 0o755)

	id := "acct_agent_save_myfile"

	off, err := StagedOffset(id)
	if err != nil || off != 0 {
		t.Fatalf("expected offset 0 for new transfer, got %d err %v", off, err)
	}

	n, err := AppendChunk(id, []byte("hello"))
	if err != nil || n != 5 {
		t.Fatalf("expected 5 bytes, got %d err %v", n, err)
	}
	n, err = AppendChunk(id, []byte("world"))
	if err != nil || n != 10 {
		t.Fatalf("expected 10 bytes, got %d err %v", n, err)
	}

	off, _ = StagedOffset(id)
	if off != 10 {
		t.Fatalf("expected staged offset 10, got %d", off)
	}

	if err := DiscardStaging(id); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if off, _ := StagedOffset(id); off != 0 {
		t.Fatalf("expected 0 after discard, got %d", off)
	}
}

func TestStagingPathNoTraversal(t *testing.T) {
	config.DataDir = t.TempDir()
	p := StagingPath("../../etc/passwd")
	if filepath.Dir(p) != filepath.Join(config.DataDir, "temp") {
		t.Fatalf("staging path escaped temp dir: %s", p)
	}
}
