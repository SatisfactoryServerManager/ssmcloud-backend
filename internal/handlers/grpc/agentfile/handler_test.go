package agentfile

import (
	"context"
	"testing"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/config"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	"google.golang.org/grpc/metadata"
)

func ctxWithKey() context.Context {
	return metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("x-api-key", "testkey"))
}

func TestGetUploadOffsetFresh(t *testing.T) {
	config.DataDir = t.TempDir()

	h := &Handler{}
	resp, err := h.GetUploadOffset(ctxWithKey(),
		&pb.UploadOffsetRequest{TransferId: "testkey_save_fresh"})
	if err != nil {
		t.Fatalf("GetUploadOffset: %v", err)
	}
	if resp.Offset != 0 {
		t.Fatalf("expected offset 0, got %d", resp.Offset)
	}
}

func TestGetUploadOffsetMissingKey(t *testing.T) {
	h := &Handler{}
	_, err := h.GetUploadOffset(context.Background(),
		&pb.UploadOffsetRequest{TransferId: "x"})
	if err == nil {
		t.Fatalf("expected error when x-api-key metadata is missing")
	}
}
