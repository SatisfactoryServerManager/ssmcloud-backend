package frontend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	accountsvc "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/account"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agentmod"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agenttask"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/integration"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/user"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/types"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/config"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	modelsV2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	pbModels "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated/models"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/utils/mapper"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedFrontendServiceServer
}

func (s *Handler) validateAPIKey(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	keys := md.Get("x-ssmcloud-key")
	if len(keys) == 0 || keys[0] != os.Getenv("SECRET_KEY") {
		return status.Error(codes.Unauthenticated, "invalid api key")
	}

	return nil
}

func (s *Handler) CheckUserExistsOrCreate(ctx context.Context, in *pb.CheckUserExistsOrCreateRequest) (*pbModels.SSMEmpty, error) {

	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, _ := user.GetUser(bson.NilObjectID, in.Eid, in.Email, in.Username)

	if theUser == nil {
		if _, err := user.CreateUser(in.Eid, in.Email, in.Username); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (s *Handler) GetUser(ctx context.Context, in *pb.GetUserRequest) (*pb.GetUserResponse, error) {

	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	pbUser := mapper.MapUserSchemaToProto(theUser)

	return &pb.GetUserResponse{User: pbUser}, nil
}

func (s *Handler) GetUserLinkedAccounts(ctx context.Context, in *pb.GetUserLinkedAccountsRequest) (*pb.GetUserLinkedAccountsResponse, error) {

	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	linkedAccounts, err := accountsvc.GetMyUserLinkedAccounts(theUser)
	if err != nil {
		return nil, err
	}

	if len(linkedAccounts) == 0 {
		return nil, nil
	}

	pbLinkedAccounts := make([]*pbModels.Account, 0, len(linkedAccounts))

	for i := range linkedAccounts {
		pbLinkedAccounts = append(pbLinkedAccounts, mapper.MapAccountSchemaToProto(&linkedAccounts[i]))
	}

	return &pb.GetUserLinkedAccountsResponse{
		LinkedAccounts: pbLinkedAccounts,
	}, nil
}

func (s *Handler) GetUserActiveAccount(ctx context.Context, in *pb.GetUserActiveAccountRequest) (*pb.GetUserActiveAccountResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err
	}

	pbActiveAccount := mapper.MapAccountSchemaToProto(activeAccount)

	return &pb.GetUserActiveAccountResponse{
		ActiveAccount: pbActiveAccount,
	}, nil
}

func (s *Handler) GetUserActiveAccountAgents(ctx context.Context, in *pb.GetUserActiveAccountAgentsRequest) (*pb.GetUserActiveAccountAgentsResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err
	}

	agents, err := agent.GetUserAccountAgents(activeAccount, bson.NilObjectID)
	if err != nil {
		return nil, err
	}

	if len(agents) == 0 {
		return nil, nil
	}

	// One aggregation for the whole list, not a count query per agent: this is the
	// dashboard's agent list and a per-agent query would be an N+1. The IDs are the
	// account's own agents, so the count stays scoped to the account being listed.
	agentIDs := make([]bson.ObjectID, 0, len(agents))
	for i := range agents {
		agentIDs = append(agentIDs, agents[i].ID)
	}

	modCounts, err := agentmod.CountDirectByAgent(agentIDs)
	if err != nil {
		return nil, err
	}

	pbAgents := make([]*pbModels.Agent, 0, len(agents))

	for i := range agents {
		pbAgent := mapper.MapAgentToProto(agents[i])
		// Agents with no mods are absent from the map; the zero value is the count.
		pbAgent.ModCount = modCounts[agents[i].ID]
		pbAgents = append(pbAgents, pbAgent)
	}

	return &pb.GetUserActiveAccountAgentsResponse{
		Agents: pbAgents,
	}, nil
}

func (s *Handler) GetUserActiveAccountUsers(ctx context.Context, in *pb.GetUserActiveAccountUsersRequest) (*pb.GetUserActiveAccountUsersResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err
	}

	users, err := accountsvc.GetUserAccountUsers(activeAccount)
	if err != nil {
		return nil, err
	}

	pbUsers := make([]*pbModels.User, 0, len(*users))

	for i := range *users {
		pbUsers = append(pbUsers, mapper.MapUserSchemaToProto(&(*users)[i]))
	}

	return &pb.GetUserActiveAccountUsersResponse{
		Users: pbUsers,
	}, nil
}

func (s *Handler) GetUserActiveAccountAudits(ctx context.Context, in *pb.GetUserActiveAccountAuditsRequest) (*pb.GetUserActiveAccountAuditsResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	audits, err := accountsvc.GetUserAccountAudit(theUser)
	if err != nil {
		return nil, err
	}

	filteredAudits := make([]modelsV2.AccountAuditSchema, 0)
	filter := modelsV2.AuditType(in.Type)

	if filter != "" {
		for _, audit := range *audits {
			if audit.Type == filter {
				filteredAudits = append(filteredAudits, audit)
			}
		}
	} else {
		filteredAudits = *audits
	}

	return &pb.GetUserActiveAccountAuditsResponse{
		Audits: mapper.MapAccountAudits(filteredAudits),
	}, nil
}

func (s *Handler) GetUserActiveAccountIntegrations(ctx context.Context, in *pb.GetUserActiveAccountIntegrationsRequest) (*pb.GetUserActiveAccountIntegrationsResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err
	}

	integrations, err := accountsvc.GetMyAccountIntegrations(activeAccount)
	if err != nil {
		return nil, err
	}

	return &pb.GetUserActiveAccountIntegrationsResponse{
		Integrations: mapper.MapAccountIntegrations(*integrations),
	}, nil
}

func (s *Handler) GetAgent(ctx context.Context, in *pb.GetAgentRequest) (*pb.GetAgentResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err
	}

	agents, err := agent.GetUserAccountAgents(activeAccount, bson.NilObjectID)
	if err != nil {
		return nil, err
	}

	if len(agents) == 0 {
		return nil, nil
	}

	for i := range agents {
		if agents[i].ID.Hex() == in.AgentId {
			return &pb.GetAgentResponse{
				Agent: mapper.MapAgentToProto(agents[i]),
			}, nil
		}
	}

	return nil, errors.New("agent not found")
}

func (s *Handler) GetAgentLog(ctx context.Context, in *pb.GetAgentLogRequest) (*pb.GetAgentLogResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	oid, err := bson.ObjectIDFromHex(in.AgentId)
	if err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := agent.GetUserAccountAgents(account, oid)
	if err != nil {
		return nil, err
	}

	if len(agents) == 0 {
		return nil, errors.New("agent not found")
	}

	theAgent := agents[0]

	theLog, err := agent.GetAgentLog(theAgent, in.Type)
	if err != nil {
		return nil, err

	}

	if in.LastIndex > int32(len(theLog.LogLines)) {
		in.LastIndex = 0
	}

	end := in.LastIndex + 500
	if end > int32(len(theLog.LogLines)) {
		end = int32(len(theLog.LogLines))
	}

	theLog.LogLines = theLog.LogLines[in.LastIndex:end]
	return &pb.GetAgentLogResponse{
		Log: mapper.MapAgentLogToProto(theLog),
	}, nil
}

func (s *Handler) GetAgentStats(ctx context.Context, in *pb.GetAgentStatsRequest) (*pb.GetAgentStatsResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	oid, err := bson.ObjectIDFromHex(in.AgentId)
	if err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := agent.GetUserAccountAgents(account, oid)
	if err != nil {
		return nil, err
	}

	if len(agents) == 0 {
		return nil, errors.New("agent not found")
	}

	theAgent := agents[0]

	stats, err := agent.GetAgentStats(theAgent)
	if err != nil {
		return nil, err
	}

	pbStats := make([]*pbModels.AgentStat, 0, len(stats))

	for i := range stats {
		pbStats = append(pbStats, mapper.MapAgentStatToProto(stats[i]))
	}

	return &pb.GetAgentStatsResponse{
		Stats: pbStats,
	}, nil
}

// resolveAgentForUser asserts the caller's active account owns the agent.
func (s *Handler) resolveAgentForUser(eid, agentID string) (*modelsV2.AgentSchema, *modelsV2.AccountSchema, error) {
	oid, err := bson.ObjectIDFromHex(agentID)
	if err != nil {
		return nil, nil, err
	}

	theUser, err := user.GetUser(bson.ObjectID{}, eid, "", "")
	if err != nil {
		return nil, nil, err
	}

	theAccount, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, nil, err
	}

	agents, err := agent.GetUserAccountAgents(theAccount, oid)
	if err != nil {
		return nil, nil, err
	}
	if len(agents) == 0 {
		return nil, nil, fmt.Errorf("agent not found")
	}

	return agents[0], theAccount, nil
}

// resolveTaskForUser asserts the caller's active account owns the task. The task
// collection is cross-account, so this is the authz boundary.
func (s *Handler) resolveTaskForUser(eid, taskID string) (*modelsV2.AgentTaskSchema, error) {
	theUser, err := user.GetUser(bson.ObjectID{}, eid, "", "")
	if err != nil {
		return nil, err
	}

	theAccount, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err
	}

	theTask, err := agenttask.Get(taskID)
	if err != nil {
		return nil, err
	}
	if theTask == nil {
		return nil, fmt.Errorf("task not found")
	}

	if theTask.AccountID != theAccount.ID {
		return nil, fmt.Errorf("task not found")
	}
	return theTask, nil
}

func (s *Handler) CreateAgentTask(ctx context.Context, in *pb.CreateAgentTaskRequest) (*pb.CreateAgentTaskResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theAgent, theAccount, err := s.resolveAgentForUser(in.Eid, in.AgentId)
	if err != nil {
		return nil, err
	}

	id, err := agent.CreateAgentTask(theAgent, theAccount, in.Eid, in.Action, nil)
	if err != nil {
		return nil, err
	}

	return &pb.CreateAgentTaskResponse{TaskId: id}, nil
}

func (s *Handler) GetAgentTasks(ctx context.Context, in *pb.GetAgentTasksRequest) (*pb.GetAgentTasksResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theAgent, _, err := s.resolveAgentForUser(in.Eid, in.AgentId)
	if err != nil {
		return nil, err
	}

	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	tasks, err := agenttask.ListForAgent(theAgent.ID, limit)
	if err != nil {
		return nil, err
	}

	res := &pb.GetAgentTasksResponse{}
	for idx := range tasks {
		t := &tasks[idx]

		view := &pb.AgentTaskView{
			Id:                    t.ID.Hex(),
			Action:                t.Action,
			Status:                t.Status,
			Progress:              t.Progress,
			Message:               t.Message,
			LastError:             t.LastError,
			Attempts:              int32(t.Attempts),
			MaxAttempts:           int32(t.MaxAttempts),
			TriggeredByType:       t.TriggeredBy.Type,
			TriggeredByExternalId: t.TriggeredBy.ExternalID,
			CreatedAt:             t.CreatedAt.Unix(),
		}
		if t.StartedAt != nil {
			view.StartedAt = t.StartedAt.Unix()
		}
		if t.FinishedAt != nil {
			view.FinishedAt = t.FinishedAt.Unix()
		}

		res.Tasks = append(res.Tasks, view)
	}

	return res, nil
}

func (s *Handler) CancelAgentTask(ctx context.Context, in *pb.CancelAgentTaskRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	if _, err := s.resolveTaskForUser(in.Eid, in.TaskId); err != nil {
		return nil, err
	}

	if err := agenttask.Cancel(in.TaskId); err != nil {
		return nil, err
	}
	return &pbModels.SSMEmpty{}, nil
}

func (s *Handler) RetryAgentTask(ctx context.Context, in *pb.RetryAgentTaskRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	if _, err := s.resolveTaskForUser(in.Eid, in.TaskId); err != nil {
		return nil, err
	}

	if err := agenttask.Retry(in.TaskId); err != nil {
		return nil, err
	}
	return &pbModels.SSMEmpty{}, nil
}


func (s *Handler) UpdateAgentSettings(ctx context.Context, in *pb.UpdateAgentSettingsRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	oid, err := bson.ObjectIDFromHex(in.AgentId)
	if err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := agent.GetUserAccountAgents(account, oid)
	if err != nil {
		return nil, err
	}

	if len(agents) == 0 {
		return nil, errors.New("agent not found")
	}

	theAgent := agents[0]

	UpdatesSettings := &types.APIUpdateServerSettings{
		ConfigSetting:        in.Settings.ConfigSetting,
		UpdateOnStart:        in.Settings.UpdateOnStart,
		AutoRestart:          in.Settings.AutoRestart,
		AutoPause:            in.Settings.AutoPause,
		AutoSaveOnDisconnect: in.Settings.AutoSaveOnDisconnect,
		AutoSaveInterval:     int(in.Settings.AutoSaveInterval),
		SeasonalEvents:       in.Settings.SeasonalEvents,
		MaxPlayers:           int(in.Settings.MaxPlayers),
		WorkerThreads:        int(in.Settings.WorkerThreads),
		Branch:               in.Settings.Branch,
		BackupInterval:       in.Settings.BackupInterval,
		BackupKeep:           int(in.Settings.BackupKeep),
		ModReference:         in.Settings.ModReference,
		ModConfig:            in.Settings.ModConfig,
	}

	if err := agent.UpdateAgentSettings(theAgent, UpdatesSettings); err != nil {
		return nil, err
	}

	return &pbModels.SSMEmpty{}, nil
}

func (s *Handler) UploadSaveFile(stream pb.FrontendService_UploadSaveFileServer) error {

	if err := s.validateAPIKey(stream.Context()); err != nil {
		logger.GetErrorLogger().Println("Error validating API key:", err)
		return err
	}

	var file *os.File
	var totalSize int64
	const maxFileSize = 1024 << 20
	var metadata *pb.FileMetadata
	var fileIdentity *types.StorageFileIdentity

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.GetErrorLogger().Println("Error receiving stream:", err)
			return err
		}

		switch data := req.Data.(type) {

		case *pb.UploadSaveFileRequest_Metadata:
			metadata = data.Metadata

			// Basic validation
			if metadata.Filename == "" {
				logger.GetWarnLogger().Println("Filename is empty")
				return stream.SendAndClose(&pb.UploadSaveFileResponse{
					Message: "filename required",
				})
			}

			normizliedFileName := filepath.Base(strings.ReplaceAll(metadata.Filename, "\\", "/"))

			uuid := utils.RandStringBytes(16)
			newFileName := uuid + "_" + normizliedFileName

			tempDir := filepath.Join(config.DataDir, "temp")

			destFilePath := filepath.Join(tempDir, newFileName)

			fileIdentity = &types.StorageFileIdentity{
				UUID:          uuid,
				FileName:      normizliedFileName,
				Extension:     filepath.Ext(normizliedFileName),
				LocalFilePath: destFilePath,
				Filesize:      0,
			}

			file, err = os.Create(destFilePath)
			if err != nil {
				logger.GetErrorLogger().Println("Error creating file:", err)
				return err
			}
			defer file.Close()

		case *pb.UploadSaveFileRequest_Chunk:
			if file == nil {
				logger.GetWarnLogger().Println("File is nil metadata not received")
				return stream.SendAndClose(&pb.UploadSaveFileResponse{
					Message: "metadata must be sent first",
				})
			}

			totalSize += int64(len(data.Chunk))
			if totalSize > maxFileSize {
				logger.GetWarnLogger().Println("File too large:", totalSize)
				return stream.SendAndClose(&pb.UploadSaveFileResponse{
					Message: "file too large",
				})
			}

			_, err := file.Write(data.Chunk)
			if err != nil {
				logger.GetErrorLogger().Println("Error writing chunk:", err)
				return err
			}
		}
	}

	oid, err := bson.ObjectIDFromHex(metadata.AgentId)
	if err != nil {
		stream.SendAndClose(&pb.UploadSaveFileResponse{
			Message: "invalid agent id",
		})
		return err
	}

	theUser, err := user.GetUser(bson.ObjectID{}, metadata.Eid, "", "")

	if err != nil {
		stream.SendAndClose(&pb.UploadSaveFileResponse{
			Message: "user not found",
		})
		return err
	}

	account, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		stream.SendAndClose(&pb.UploadSaveFileResponse{
			Message: "account not found",
		})
		return err

	}

	agents, err := agent.GetUserAccountAgents(account, oid)
	if err != nil {
		stream.SendAndClose(&pb.UploadSaveFileResponse{
			Message: "error getting agents",
		})
		return err
	}

	if len(agents) == 0 {
		stream.SendAndClose(&pb.UploadSaveFileResponse{
			Message: "agent not found",
		})
		return errors.New("agent not found")
	}

	theAgent := agents[0]

	fileIdentity.Filesize = totalSize

	err = agent.UploadedAgentSave(theAgent.APIKey, *fileIdentity, true)
	if err != nil {
		stream.SendAndClose(&pb.UploadSaveFileResponse{
			Message: "error uploading save file",
		})
		return err
	}

	return stream.SendAndClose(&pb.UploadSaveFileResponse{
		Message: "upload successful",
	})
}

func (s *Handler) CreateAgent(ctx context.Context, in *pb.CreateAgentRequest) (*pb.CreateAgentResponse, error) {

	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err

	}
	workflowData := &modelsV2.CreateAgentWorkflowData{
		AgentName:  in.AgentName,
		Port:       int(in.Port),
		Memory:     int64(in.Memory),
		AdminPass:  in.AdminPass,
		ClientPass: in.ClientPass,
		APIKey:     in.ApiKey,
	}

	workflowId, err := agent.NewWorkflow_CreateAgent(account.ID, workflowData)
	if err != nil {
		return nil, err
	}

	return &pb.CreateAgentResponse{
		WorkflowId: workflowId,
	}, nil
}

func (s *Handler) DeleteAgent(ctx context.Context, in *pb.DeleteAgentRequest) (*pbModels.SSMEmpty, error) {

	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	oid, err := bson.ObjectIDFromHex(in.AgentId)
	if err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err

	}

	if err := agent.DeleteAgent(account, oid); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *Handler) SwitchActiveAccount(ctx context.Context, in *pb.SwitchActiveAccountRequest) (*pbModels.SSMEmpty, error) {

	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	oid, err := bson.ObjectIDFromHex(in.AccountId)
	if err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.ObjectID{}, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	if err := accountsvc.SwitchAccount(theUser, oid); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *Handler) CreateAccount(ctx context.Context, in *pb.CreateAccountRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.ObjectID{}, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	if err := accountsvc.CreateAccount(theUser, in.AccountName); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *Handler) JoinAccount(ctx context.Context, in *pb.JoinAccountRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.ObjectID{}, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	if err := accountsvc.JoinAccount(theUser, in.JoinCode); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *Handler) DeleteAccount(ctx context.Context, in *pb.DeleteAccountRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := user.GetUser(bson.ObjectID{}, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	oid, err := bson.ObjectIDFromHex(in.AccountId)
	if err != nil {
		return nil, err
	}

	if err := accountsvc.DeleteAccount(theUser, oid); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *Handler) GetAgentWorkflow(ctx context.Context, in *pb.GetAgentWorkflowRequest) (*pb.GetAgentWorkflowResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	WorkflowModel, err := repositories.GetMongoClient().GetModel("Workflow")
	if err != nil {
		return nil, err
	}

	WorkflowId, err := bson.ObjectIDFromHex(in.WorkflowId)

	if err != nil {
		return nil, err
	}

	theWorkflow := &modelsV2.WorkflowSchema{}

	if err := WorkflowModel.FindOne(theWorkflow, bson.M{"_id": WorkflowId}); err != nil {
		return nil, err
	}

	return &pb.GetAgentWorkflowResponse{
		Workflow: mapper.MapWorkflowToProto(theWorkflow),
	}, nil
}

func (s *Handler) GetAgentWorkflowByAgent(ctx context.Context, in *pb.GetAgentWorkflowByAgentRequest) (*pb.GetAgentWorkflowResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	WorkflowModel, err := repositories.GetMongoClient().GetModel("Workflow")
	if err != nil {
		return nil, err
	}

	AgentId, err := bson.ObjectIDFromHex(in.AgentId)

	if err != nil {
		return nil, err
	}

	theWorkflow := &modelsV2.WorkflowSchema{}

	if err := WorkflowModel.FindOne(theWorkflow, bson.M{"agentId": AgentId}); err != nil {
		// Agents created before workflows existed have none. Not an error.
		if errors.Is(err, mongo.ErrNoDocuments) {
			return &pb.GetAgentWorkflowResponse{}, nil
		}
		return nil, err
	}

	return &pb.GetAgentWorkflowResponse{
		Workflow: mapper.MapWorkflowToProto(theWorkflow),
	}, nil
}

func (s *Handler) GetAccountIntegrationEvents(ctx context.Context, in *pb.GetAccountIntegrationEventsRequest) (*pb.GetAccountIntegrationEventsResponse, error) {

	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	integrationId, err := bson.ObjectIDFromHex(in.IntegrationId)
	if err != nil {
		return nil, err
	}

	events, err := integration.GetMyAccountIntegrationsEvents(integrationId)
	if err != nil {
		return nil, err
	}

	return &pb.GetAccountIntegrationEventsResponse{
		Events: mapper.MapIntegrationEventsToProto(events),
	}, nil
}
