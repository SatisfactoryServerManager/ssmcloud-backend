package frontend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/v2"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/types"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/config"
	modelsV2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	pbModels "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated/models"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/utils/mapper"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

	theUser, _ := v2.GetUser(primitive.NilObjectID, in.Eid, in.Email, in.Username)

	if theUser == nil {
		if _, err := v2.CreateUser(in.Eid, in.Email, in.Username); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (s *Handler) GetUser(ctx context.Context, in *pb.GetUserRequest) (*pb.GetUserResponse, error) {

	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.NilObjectID, in.Eid, "", "")
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

	theUser, err := v2.GetUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	linkedAccounts, err := v2.GetMyUserLinkedAccounts(theUser)
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

	theUser, err := v2.GetUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := v2.GetUserActiveAccount(theUser)
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

	theUser, err := v2.GetUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err
	}

	agents, err := v2.GetUserAccountAgents(activeAccount, primitive.NilObjectID)
	if err != nil {
		return nil, err
	}

	if len(agents) == 0 {
		return nil, nil
	}

	pbAgents := make([]*pbModels.Agent, 0, len(agents))

	for i := range agents {
		pbAgents = append(pbAgents, mapper.MapAgentToProto(agents[i]))
	}

	return &pb.GetUserActiveAccountAgentsResponse{
		Agents: pbAgents,
	}, nil
}

func (s *Handler) GetUserActiveAccountUsers(ctx context.Context, in *pb.GetUserActiveAccountUsersRequest) (*pb.GetUserActiveAccountUsersResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err
	}

	users, err := v2.GetUserAccountUsers(activeAccount)
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

	theUser, err := v2.GetUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	audits, err := v2.GetUserAccountAudit(theUser)
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

	theUser, err := v2.GetUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err
	}

	integrations, err := v2.GetMyAccountIntegrations(activeAccount)
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

	theUser, err := v2.GetUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err
	}

	agents, err := v2.GetUserAccountAgents(activeAccount, primitive.NilObjectID)
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

	oid, err := primitive.ObjectIDFromHex(in.AgentId)
	if err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetUserAccountAgents(account, oid)
	if err != nil {
		return nil, err
	}

	if len(agents) == 0 {
		return nil, errors.New("agent not found")
	}

	theAgent := agents[0]

	theLog, err := v2.GetAgentLog(theAgent, in.Type)
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

	oid, err := primitive.ObjectIDFromHex(in.AgentId)
	if err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetUserAccountAgents(account, oid)
	if err != nil {
		return nil, err
	}

	if len(agents) == 0 {
		return nil, errors.New("agent not found")
	}

	theAgent := agents[0]

	stats, err := v2.GetAgentStats(theAgent)
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

func (s *Handler) CreateAgentTask(ctx context.Context, in *pb.CreateAgentTaskRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	oid, err := primitive.ObjectIDFromHex(in.AgentId)
	if err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetUserAccountAgents(account, oid)
	if err != nil {
		return nil, err
	}

	if len(agents) == 0 {
		return nil, errors.New("agent not found")
	}

	theAgent := agents[0]

	if err := v2.CreateAgentTask(theAgent, in.Action, nil); err != nil {
		return nil, err
	}

	return &pbModels.SSMEmpty{}, nil
}

func (s *Handler) GetAgentMods(ctx context.Context, in *pb.GetAgentModsRequest) (*pb.GetAgentModsResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	oid, err := primitive.ObjectIDFromHex(in.AgentId)
	if err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetUserAccountAgents(account, oid)
	if err != nil {
		return nil, err
	}

	if len(agents) == 0 {
		return nil, errors.New("agent not found")
	}

	theAgent := agents[0]

	mods, err := v2.GetModsFromDB(int(in.Page), in.Sort, in.Direction, in.Search)
	if err != nil {
		return nil, err
	}
	modCount, err := v2.GetDBModCount()
	if err != nil {
		return nil, err
	}

	pages := float64(modCount) / float64(30)

	ModModel, err := repositories.GetMongoClient().GetModel("AgentModConfigSelectedMod")
	if err != nil {
		return nil, err
	}

	for idx := range theAgent.ModConfig.SelectedMods {
		mod := &theAgent.ModConfig.SelectedMods[idx]
		if err := ModModel.PopulateField(mod, "Mod"); err != nil {
			return nil, fmt.Errorf("error populating mod with error: %s", err.Error())
		}
	}

	pbMods := make([]*pbModels.Mod, 0, len(*mods))

	for i := range *mods {
		mod := &(*mods)[i]
		pbMods = append(pbMods, mapper.MapModToProto(mod))
	}

	return &pb.GetAgentModsResponse{
		Mods:           pbMods,
		AgentModConfig: mapper.MapAgentModConfigToProto(&theAgent.ModConfig),
		TotalMods:      int32(modCount),
		Pages:          int32(math.Ceil(pages)),
	}, nil
}

func (s *Handler) InstallAgentMod(ctx context.Context, in *pb.InstallAgentModRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	oid, err := primitive.ObjectIDFromHex(in.AgentId)
	if err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetUserAccountAgents(account, oid)
	if err != nil {
		return nil, err
	}

	if len(agents) == 0 {
		return nil, errors.New("agent not found")
	}

	theAgent := agents[0]

	if err := v2.UpdateMod(theAgent, in.ModReference); err != nil {
		return nil, err
	}

	return &pbModels.SSMEmpty{}, nil
}

func (s *Handler) UninstallAgentMod(ctx context.Context, in *pb.UninstallAgentModRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	oid, err := primitive.ObjectIDFromHex(in.AgentId)
	if err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetUserAccountAgents(account, oid)
	if err != nil {
		return nil, err
	}

	if len(agents) == 0 {
		return nil, errors.New("agent not found")
	}

	theAgent := agents[0]

	if err := v2.UninstallMod(theAgent, in.ModReference); err != nil {
		return nil, err
	}

	return &pbModels.SSMEmpty{}, nil
}

func (s *Handler) UpdateAgentSettings(ctx context.Context, in *pb.UpdateAgentSettingsRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	oid, err := primitive.ObjectIDFromHex(in.AgentId)
	if err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetUserAccountAgents(account, oid)
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

	if err := v2.UpdateAgentSettings(theAgent, UpdatesSettings); err != nil {
		return nil, err
	}

	return &pbModels.SSMEmpty{}, nil
}

func (s *Handler) UploadSaveFile(stream pb.FrontendService_UploadSaveFileServer) error {

	if err := s.validateAPIKey(stream.Context()); err != nil {
		fmt.Println("Error validating API key:", err)
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
			fmt.Println("Error receiving stream:", err)
			return err
		}

		switch data := req.Data.(type) {

		case *pb.UploadSaveFileRequest_Metadata:
			metadata = data.Metadata

			// Basic validation
			if metadata.Filename == "" {
				fmt.Println("Filename is empty")
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
				fmt.Println("Error creating file:", err)
				return err
			}
			defer file.Close()

		case *pb.UploadSaveFileRequest_Chunk:
			if file == nil {
				fmt.Println("File is nil metadata not received")
				return stream.SendAndClose(&pb.UploadSaveFileResponse{
					Message: "metadata must be sent first",
				})
			}

			totalSize += int64(len(data.Chunk))
			if totalSize > maxFileSize {
				fmt.Println("File too large:", totalSize)
				return stream.SendAndClose(&pb.UploadSaveFileResponse{
					Message: "file too large",
				})
			}

			_, err := file.Write(data.Chunk)
			if err != nil {
				fmt.Println("Error writing chunk:", err)
				return err
			}
		}
	}

	oid, err := primitive.ObjectIDFromHex(metadata.AgentId)
	if err != nil {
		stream.SendAndClose(&pb.UploadSaveFileResponse{
			Message: "invalid agent id",
		})
		return err
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, metadata.Eid, "", "")

	if err != nil {
		stream.SendAndClose(&pb.UploadSaveFileResponse{
			Message: "user not found",
		})
		return err
	}

	account, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		stream.SendAndClose(&pb.UploadSaveFileResponse{
			Message: "account not found",
		})
		return err

	}

	agents, err := v2.GetUserAccountAgents(account, oid)
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

	theUser, err := v2.GetUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetUserActiveAccount(theUser)
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

	workflowId, err := v2.NewWorkflow_CreateAgent(account.ID, workflowData)
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

	oid, err := primitive.ObjectIDFromHex(in.AgentId)
	if err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err

	}

	if err := v2.DeleteAgent(account, oid); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *Handler) SwitchActiveAccount(ctx context.Context, in *pb.SwitchActiveAccountRequest) (*pbModels.SSMEmpty, error) {

	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	oid, err := primitive.ObjectIDFromHex(in.AccountId)
	if err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	if err := v2.SwitchAccount(theUser, oid); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *Handler) CreateAccount(ctx context.Context, in *pb.CreateAccountRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	if err := v2.CreateAccount(theUser, in.AccountName); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *Handler) JoinAccount(ctx context.Context, in *pb.JoinAccountRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	if err := v2.JoinAccount(theUser, in.JoinCode); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *Handler) DeleteAccount(ctx context.Context, in *pb.DeleteAccountRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	oid, err := primitive.ObjectIDFromHex(in.AccountId)
	if err != nil {
		return nil, err
	}

	if err := v2.DeleteAccount(theUser, oid); err != nil {
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

	WorkflowId, err := primitive.ObjectIDFromHex(in.WorkflowId)

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

func (s *Handler) GetAccountIntegrationEvents(ctx context.Context, in *pb.GetAccountIntegrationEventsRequest) (*pb.GetAccountIntegrationEventsResponse, error) {

	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	integrationId, err := primitive.ObjectIDFromHex(in.IntegrationId)
	if err != nil {
		return nil, err
	}

	events, err := v2.GetMyAccountIntegrationsEvents(integrationId)
	if err != nil {
		return nil, err
	}

	return &pb.GetAccountIntegrationEventsResponse{
		Events: mapper.MapIntegrationEventsToProto(events),
	}, nil
}
