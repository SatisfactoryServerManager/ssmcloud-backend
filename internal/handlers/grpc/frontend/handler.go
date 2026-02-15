package frontend

import (
	"context"
	"errors"
	"os"

	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/v2"
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

	pbLinkedAccounts := make([]*pb.Account, 0, len(linkedAccounts))

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
