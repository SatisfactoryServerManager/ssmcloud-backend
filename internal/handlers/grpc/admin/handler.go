package admin

import (
	"context"
	"os"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/admin"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	pbModels "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated/models"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/utils/mapper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedAdminServiceServer
}

func (h *Handler) validateAdminKey(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	keys := md.Get("x-ssmcloud-admin-key")
	if len(keys) == 0 {
		// Backwards-compatible fallback
		keys = md.Get("x-ssmcloud-key")
	}

	expected := os.Getenv("ADMIN_SECRET_KEY")
	if expected == "" {
		expected = os.Getenv("SECRET_KEY")
	}

	if expected == "" || len(keys) == 0 || keys[0] != expected {
		return status.Error(codes.Unauthenticated, "invalid admin key")
	}

	return nil
}

// ---- Users ----

func (h *Handler) GetUser(ctx context.Context, in *pb.AdminGetUserRequest) (*pb.AdminGetUserResponse, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	u, err := admin.AdminGetUser(in.UserId, in.ExternalId, in.Email)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pb.AdminGetUserResponse{User: mapper.MapUserSchemaToProto(u)}, nil
}

func (h *Handler) ListUsers(ctx context.Context, in *pb.AdminListUsersRequest) (*pb.AdminListUsersResponse, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	users, total, err := admin.AdminListUsers(in.Page, in.PageSize, in.Search)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	out := make([]*pbModels.User, 0, len(users))
	for i := range users {
		out = append(out, mapper.MapUserSchemaToProto(&users[i]))
	}

	return &pb.AdminListUsersResponse{Users: out, Total: int32(total)}, nil
}

func (h *Handler) UpdateUser(ctx context.Context, in *pb.AdminUpdateUserRequest) (*pb.AdminUpdateUserResponse, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	u, err := admin.AdminUpdateUser(in.UserId, in.ExternalId, in.Email, in.Username)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pb.AdminUpdateUserResponse{User: mapper.MapUserSchemaToProto(u)}, nil
}

func (h *Handler) DeleteUser(ctx context.Context, in *pb.AdminDeleteUserRequest) (*pbModels.SSMEmpty, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	if err := admin.AdminDeleteUser(in.UserId); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pbModels.SSMEmpty{}, nil
}

// ---- Accounts ----

func (h *Handler) GetAccount(ctx context.Context, in *pb.AdminGetAccountRequest) (*pb.AdminGetAccountResponse, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	a, err := admin.AdminGetAccount(in.AccountId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pb.AdminGetAccountResponse{Account: mapper.MapAccountSchemaToProto(a)}, nil
}

func (h *Handler) ListAccounts(ctx context.Context, in *pb.AdminListAccountsRequest) (*pb.AdminListAccountsResponse, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	accounts, total, err := admin.AdminListAccounts(in.Page, in.PageSize, in.Search)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	out := make([]*pbModels.Account, 0, len(accounts))
	for i := range accounts {
		out = append(out, mapper.MapAccountSchemaToProto(&accounts[i]))
	}

	return &pb.AdminListAccountsResponse{Accounts: out, Total: int32(total)}, nil
}

func (h *Handler) UpdateAccount(ctx context.Context, in *pb.AdminUpdateAccountRequest) (*pb.AdminUpdateAccountResponse, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	a, err := admin.AdminUpdateAccount(in.AccountId, in.AccountName)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pb.AdminUpdateAccountResponse{Account: mapper.MapAccountSchemaToProto(a)}, nil
}

func (h *Handler) DeleteAccount(ctx context.Context, in *pb.AdminDeleteAccountRequest) (*pbModels.SSMEmpty, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	if err := admin.AdminDeleteAccount(in.AccountId); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pbModels.SSMEmpty{}, nil
}

// ---- Agents ----

func (h *Handler) GetAgent(ctx context.Context, in *pb.AdminGetAgentRequest) (*pb.AdminGetAgentResponse, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	agent, err := admin.AdminGetAgent(in.AgentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pb.AdminGetAgentResponse{Agent: mapper.MapAgentToProto(agent)}, nil
}

func (h *Handler) ListAgents(ctx context.Context, in *pb.AdminListAgentsRequest) (*pb.AdminListAgentsResponse, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	agents, total, err := admin.AdminListAgents(in.Page, in.PageSize, in.Search)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	out := make([]*pbModels.Agent, 0, len(agents))
	for i := range agents {
		out = append(out, mapper.MapAgentToProto(&agents[i]))
	}

	return &pb.AdminListAgentsResponse{Agents: out, Total: int32(total)}, nil
}

func (h *Handler) UpdateAgent(ctx context.Context, in *pb.AdminUpdateAgentRequest) (*pb.AdminUpdateAgentResponse, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	agent, err := admin.AdminUpdateAgent(in.AgentId, in.AgentName, in.ApiKey)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pb.AdminUpdateAgentResponse{Agent: mapper.MapAgentToProto(agent)}, nil
}

func (h *Handler) DeleteAgent(ctx context.Context, in *pb.AdminDeleteAgentRequest) (*pbModels.SSMEmpty, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	if err := admin.AdminDeleteAgent(in.AgentId); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pbModels.SSMEmpty{}, nil
}

// ---- Relationships ----

func (h *Handler) AddUserToAccount(ctx context.Context, in *pb.AdminAddUserToAccountRequest) (*pbModels.SSMEmpty, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	if err := admin.AdminAddUserToAccount(in.UserId, in.AccountId, in.SetActive); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pbModels.SSMEmpty{}, nil
}

func (h *Handler) ListUserAccounts(ctx context.Context, in *pb.AdminListUserAccountsRequest) (*pb.AdminListUserAccountsResponse, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	accounts, activeId, err := admin.AdminListUserAccounts(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	out := make([]*pbModels.Account, 0, len(accounts))
	for i := range accounts {
		out = append(out, mapper.MapAccountSchemaToProto(&accounts[i]))
	}

	return &pb.AdminListUserAccountsResponse{Accounts: out, ActiveAccountId: activeId}, nil
}

func (h *Handler) RemoveUserFromAccount(ctx context.Context, in *pb.AdminRemoveUserFromAccountRequest) (*pbModels.SSMEmpty, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	if err := admin.AdminRemoveUserFromAccount(in.UserId, in.AccountId); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pbModels.SSMEmpty{}, nil
}

func (h *Handler) SetUserActiveAccount(ctx context.Context, in *pb.AdminSetUserActiveAccountRequest) (*pbModels.SSMEmpty, error) {
	if err := h.validateAdminKey(ctx); err != nil {
		return nil, err
	}

	if err := admin.AdminSetUserActiveAccount(in.UserId, in.AccountId); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pbModels.SSMEmpty{}, nil
}
