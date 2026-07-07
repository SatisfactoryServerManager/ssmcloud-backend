package agentfile

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/handlers/grpc/transfer"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/types"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	pbModels "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated/models"
)

type Handler struct {
	pb.UnimplementedAgentFileServiceServer
}

func saveToPB(s modelsv2.AgentSave) *pb.SaveSyncItem {
	return &pb.SaveSyncItem{
		Uuid:        s.UUID,
		FileName:    s.FileName,
		Size:        s.Size,
		ModTimeUnix: s.ModTime.Unix(),
	}
}

func pbToSave(s *pb.SaveSyncItem) modelsv2.AgentSave {
	return modelsv2.AgentSave{
		UUID:     s.Uuid,
		FileName: s.FileName,
		Size:     s.Size,
		ModTime:  time.Unix(s.ModTimeUnix, 0).UTC(),
	}
}

func (h *Handler) GetUploadOffset(ctx context.Context, in *pb.UploadOffsetRequest) (*pb.UploadOffsetResponse, error) {
	if _, err := utils.GetAPIKeyFromContext(ctx); err != nil {
		return nil, err
	}
	off, err := transfer.StagedOffset(in.TransferId)
	if err != nil {
		return nil, err
	}
	return &pb.UploadOffsetResponse{Offset: off}, nil
}

func (h *Handler) UploadFile(stream pb.AgentFileService_UploadFileServer) error {
	apiKey, err := utils.GetAPIKeyFromContext(stream.Context())
	if err != nil {
		return err
	}

	var init *pb.UploadInit
	for {
		req, rerr := stream.Recv()
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
		switch data := req.Data.(type) {
		case *pb.UploadFileRequest_Init:
			init = data.Init
		case *pb.UploadFileRequest_Chunk:
			if init == nil {
				return stream.SendAndClose(&pb.UploadFileResponse{Success: false, Message: "init must be sent first"})
			}
			if _, aerr := transfer.AppendChunk(init.TransferId, data.Chunk); aerr != nil {
				return aerr
			}
		}
	}
	if init == nil {
		return stream.SendAndClose(&pb.UploadFileResponse{Success: false, Message: "no init received"})
	}

	fileIdentity := types.StorageFileIdentity{
		FileName:      filepath.Base(init.Filename),
		Extension:     filepath.Ext(init.Filename),
		LocalFilePath: transfer.FinalizePath(init.TransferId),
	}

	switch init.Kind {
	case pb.FileKind_FILE_KIND_SAVE:
		err = agent.UploadedAgentSave(*apiKey, fileIdentity, false)
	case pb.FileKind_FILE_KIND_BACKUP:
		err = agent.UploadedAgentBackup(*apiKey, fileIdentity)
	case pb.FileKind_FILE_KIND_LOG:
		err = agent.UploadedAgentLog(*apiKey, fileIdentity)
	default:
		err = fmt.Errorf("unknown file kind %v", init.Kind)
	}
	if err != nil {
		_ = transfer.DiscardStaging(init.TransferId)
		return stream.SendAndClose(&pb.UploadFileResponse{Success: false, Message: err.Error()})
	}
	// UploadAgentFile removes LocalFilePath on success; ensure staging cleaned.
	_ = transfer.DiscardStaging(init.TransferId)
	return stream.SendAndClose(&pb.UploadFileResponse{Success: true})
}

func (h *Handler) DownloadFile(in *pb.DownloadFileRequest, stream pb.AgentFileService_DownloadFileServer) error {
	apiKey, err := utils.GetAPIKeyFromContext(stream.Context())
	if err != nil {
		return err
	}
	objectPath, err := agent.SaveObjectPathForAPIKey(*apiKey, in.Filename)
	if err != nil {
		return err
	}
	obj, err := repositories.GetAgentFileRange(objectPath, in.StartOffset)
	if err != nil {
		return err
	}
	defer obj.Body.Close()

	buf := make([]byte, transfer.ChunkSize)
	for {
		n, rerr := obj.Body.Read(buf)
		if n > 0 {
			if serr := stream.Send(&pb.DownloadChunk{Chunk: buf[:n]}); serr != nil {
				return serr
			}
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

func (h *Handler) GetSaveSync(ctx context.Context, _ *pbModels.SSMEmpty) (*pb.GetSaveSyncResponse, error) {
	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}
	saves, err := agent.GetAgentSaves(*apiKey)
	if err != nil {
		return nil, err
	}
	out := &pb.GetSaveSyncResponse{}
	for i := range saves {
		out.Saves = append(out.Saves, saveToPB(saves[i]))
	}
	return out, nil
}

func (h *Handler) PostSaveSync(ctx context.Context, in *pb.PostSaveSyncRequest) (*pbModels.SSMEmpty, error) {
	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}
	saves := make([]modelsv2.AgentSave, 0, len(in.Saves))
	for _, s := range in.Saves {
		saves = append(saves, pbToSave(s))
	}
	if err := agent.PostAgentSyncSaves(*apiKey, saves); err != nil {
		return nil, err
	}
	return &pbModels.SSMEmpty{}, nil
}
