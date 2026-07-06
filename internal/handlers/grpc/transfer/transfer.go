package transfer

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/config"
)

const ChunkSize = 64 * 1024

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9_.-]`)

func stagingDir() string { return filepath.Join(config.DataDir, "temp") }

// StagingPath maps a transfer id to a single flat file inside the temp dir.
// All path separators and unsafe characters are stripped so a transfer id can
// never escape the staging directory.
func StagingPath(transferID string) string {
	safe := unsafeChars.ReplaceAllString(filepath.Base(transferID), "_")
	return filepath.Join(stagingDir(), safe+".part")
}

func StagedOffset(transferID string) (int64, error) {
	fi, err := os.Stat(StagingPath(transferID))
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func AppendChunk(transferID string, chunk []byte) (int64, error) {
	if err := os.MkdirAll(stagingDir(), 0o755); err != nil {
		return 0, err
	}
	f, err := os.OpenFile(StagingPath(transferID), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if _, err := f.Write(chunk); err != nil {
		return 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func FinalizePath(transferID string) string { return StagingPath(transferID) }

func DiscardStaging(transferID string) error {
	err := os.Remove(StagingPath(transferID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
