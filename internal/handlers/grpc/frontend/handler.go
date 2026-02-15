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

	theUser, _ := v2.GetMyUser(primitive.NilObjectID, in.Eid, in.Email, in.Username)

	if theUser == nil {
		if _, err := v2.CreateUser(in.Eid, in.Email, in.Username); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (s *Handler) GetMyUser(ctx context.Context, in *pb.GetMyUserRequest) (*pb.GetMyUserResponse, error) {

	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := v2.GetMyUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	pbUser := mapper.MapUserSchemaToProto(theUser)

	return &pb.GetMyUserResponse{User: pbUser}, nil
}

func (s *Handler) GetMyUserLinkedAccounts(ctx context.Context, in *pb.GetMyUserLinkedAccountsRequest) (*pb.GetMyUserLinkedAccountsResponse, error) {

	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := v2.GetMyUser(primitive.NilObjectID, in.Eid, "", "")
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

	return &pb.GetMyUserLinkedAccountsResponse{
		LinkedAccounts: pbLinkedAccounts,
	}, nil
}

func (s *Handler) GetMyUserActiveAccount(ctx context.Context, in *pb.GetMyUserActiveAccountRequest) (*pb.GetMyUserActiveAccountResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := v2.GetMyUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		return nil, err
	}

	pbActiveAccount := mapper.MapAccountSchemaToProto(activeAccount)

	return &pb.GetMyUserActiveAccountResponse{
		ActiveAccount: pbActiveAccount,
	}, nil
}

func (s *Handler) GetMyUserActiveAccountAgents(ctx context.Context, in *pb.GetMyUserActiveAccountAgentsRequest) (*pb.GetMyUserActiveAccountAgentsResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := v2.GetMyUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		return nil, err
	}

	agents, err := v2.GetMyUserAccountAgents(activeAccount, primitive.NilObjectID)
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

	return &pb.GetMyUserActiveAccountAgentsResponse{
		Agents: pbAgents,
	}, nil
}

func (s *Handler) GetMyUserActiveAccountUsers(ctx context.Context, in *pb.GetMyUserActiveAccountUsersRequest) (*pb.GetMyUserActiveAccountUsersResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := v2.GetMyUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		return nil, err
	}

	users, err := v2.GetMyAccountUsers(activeAccount)
	if err != nil {
		return nil, err
	}

	pbUsers := make([]*pbModels.User, 0, len(*users))

	for i := range *users {
		pbUsers = append(pbUsers, mapper.MapUserSchemaToProto(&(*users)[i]))
	}

	return &pb.GetMyUserActiveAccountUsersResponse{
		Users: pbUsers,
	}, nil
}

func (s *Handler) GetMyUserActiveAccountAudits(ctx context.Context, in *pb.GetMyUserActiveAccountAuditsRequest) (*pb.GetMyUserActiveAccountAuditsResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := v2.GetMyUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	audits, err := v2.GetMyAccountAudit(theUser)
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

	return &pb.GetMyUserActiveAccountAuditsResponse{
		Audits: mapper.MapAccountAudits(filteredAudits),
	}, nil
}

func (s *Handler) GetMyUserActiveAccountSingleAgent(ctx context.Context, in *pb.GetMyUserActiveAccountSingleAgentRequest) (*pb.GetMyUserActiveAccountSingleAgentResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theUser, err := v2.GetMyUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		return nil, err
	}

	agents, err := v2.GetMyUserAccountAgents(activeAccount, primitive.NilObjectID)
	if err != nil {
		return nil, err
	}

	if len(agents) == 0 {
		return nil, nil
	}

	for i := range agents {
		if agents[i].ID.Hex() == in.AgentId {
			return &pb.GetMyUserActiveAccountSingleAgentResponse{
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

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetMyUserAccountAgents(account, oid)
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

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetMyUserAccountAgents(account, oid)
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

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetMyUserAccountAgents(account, oid)
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

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetMyUserAccountAgents(account, oid)
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

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetMyUserAccountAgents(account, oid)
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

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetMyUserAccountAgents(account, oid)
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

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, in.Eid, "", "")

	if err != nil {
		return nil, err
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		return nil, err

	}

	agents, err := v2.GetMyUserAccountAgents(account, oid)
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

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, metadata.Eid, "", "")

	if err != nil {
		stream.SendAndClose(&pb.UploadSaveFileResponse{
			Message: "user not found",
		})
		return err
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		stream.SendAndClose(&pb.UploadSaveFileResponse{
			Message: "account not found",
		})
		return err

	}

	agents, err := v2.GetMyUserAccountAgents(account, oid)
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
