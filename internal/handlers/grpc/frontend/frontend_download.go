package frontend

import (
	"context"
	"fmt"
	"io"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/handlers/grpc/transfer"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	accountsvc "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/account"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/user"
	modelsV2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	pbModels "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func frontendObjectPath(accountID, agentID, subdir, filename string) string {
	return fmt.Sprintf("%s/%s/%s/%s", accountID, agentID, subdir, filename)
}

// resolveDownload maps a FrontendDownloadRequest to the S3 object path for the
// requested file, mirroring the old REST download handlers.
func resolveDownload(in *pb.FrontendDownloadRequest) (string, string, error) {
	theUser, err := user.GetUser(bson.NilObjectID, in.Eid, "", "")
	if err != nil {
		return "", "", err
	}
	theAccount, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return "", "", err
	}
	oid, err := bson.ObjectIDFromHex(in.AgentId)
	if err != nil {
		return "", "", fmt.Errorf("invalid agent id")
	}
	agents, err := agent.GetUserAccountAgents(theAccount, oid)
	if err != nil || len(agents) == 0 {
		return "", "", fmt.Errorf("agent not found")
	}
	theAgent := agents[0]

	var subdir, filename string
	switch in.Kind {
	case pb.FrontendDownloadKind_FRONTEND_DOWNLOAD_SAVE:
		subdir = "saves"
		for i := range theAgent.Saves {
			if theAgent.Saves[i].UUID == in.Uuid {
				filename = theAgent.Saves[i].FileName
				break
			}
		}
	case pb.FrontendDownloadKind_FRONTEND_DOWNLOAD_BACKUP:
		subdir = "backups"
		for i := range theAgent.Backups {
			if theAgent.Backups[i].UUID == in.Uuid {
				filename = theAgent.Backups[i].FileName
				break
			}
		}
	case pb.FrontendDownloadKind_FRONTEND_DOWNLOAD_LOG:
		subdir = "logs"
		AgentModel, merr := repositories.GetMongoClient().GetModel("Agent")
		if merr != nil {
			return "", "", merr
		}
		if perr := AgentModel.PopulateField(theAgent, "Logs"); perr != nil {
			return "", "", perr
		}
		for i := range theAgent.Logs {
			if theAgent.Logs[i].Type == in.Logtype {
				filename = theAgent.Logs[i].FileName
				break
			}
		}
	default:
		return "", "", fmt.Errorf("unknown download kind")
	}

	if filename == "" {
		return "", "", fmt.Errorf("file not found")
	}

	return frontendObjectPath(theAccount.ID.Hex(), theAgent.ID.Hex(), subdir, filename), filename, nil
}

func (s *Handler) DownloadFile(in *pb.FrontendDownloadRequest, stream pb.FrontendService_DownloadFileServer) error {
	if err := s.validateAPIKey(stream.Context()); err != nil {
		return err
	}

	objectPath, _, err := resolveDownload(in)
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
			if serr := stream.Send(&pb.DownloadFileChunk{Chunk: buf[:n]}); serr != nil {
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

func toEventTypes(in []string) []modelsV2.IntegrationEventType {
	out := make([]modelsV2.IntegrationEventType, 0, len(in))
	for _, e := range in {
		out = append(out, modelsV2.IntegrationEventType(e))
	}
	return out
}

func (s *Handler) AddAccountIntegration(ctx context.Context, in *pb.AddAccountIntegrationRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}
	theUser, err := user.GetUser(bson.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}
	theAccount, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err
	}
	if err := accountsvc.AddAccountIntegration(theAccount, in.Name, modelsV2.IntegrationType(in.Type), in.Url, toEventTypes(in.EventTypes)); err != nil {
		return nil, err
	}
	return &pbModels.SSMEmpty{}, nil
}

func (s *Handler) UpdateAccountIntegration(ctx context.Context, in *pb.UpdateAccountIntegrationRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}
	integrationId, err := bson.ObjectIDFromHex(in.IntegrationId)
	if err != nil {
		return nil, fmt.Errorf("invalid integration id")
	}
	if err := accountsvc.UpdateAccountIntegration(integrationId, in.Name, modelsV2.IntegrationType(in.Type), in.Url, toEventTypes(in.EventTypes)); err != nil {
		return nil, err
	}
	return &pbModels.SSMEmpty{}, nil
}
