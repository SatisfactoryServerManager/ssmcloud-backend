package frontend

import (
	"context"

	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/v2"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/utils/mapper"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Handler struct {
	pb.UnimplementedFrontendServiceServer
}

func (s *Handler) CheckUserExistsOrCreate(ctx context.Context, in *pb.CheckUserExistsOrCreateRequest) (*pb.Empty, error) {

	theUser, _ := v2.GetMyUser(primitive.NilObjectID, in.Eid, in.Email, in.Username)

	if theUser == nil {
		if _, err := v2.CreateUser(in.Eid, in.Email, in.Username); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (s *Handler) GetMyUser(ctx context.Context, in *pb.GetMyUserRequest) (*pb.GetMyUserResponse, error) {

	theUser, err := v2.GetMyUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	pbUser := mapper.MapUserSchemaToProto(theUser)

	return &pb.GetMyUserResponse{User: pbUser}, nil
}

func (s *Handler) GetMyUserLinkedAccounts(ctx context.Context, in *pb.GetMyUserLinkedAccountsRequest) (*pb.GetMyUserLinkedAccountsResponse, error) {
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

func (s *Handler) GetMyUserActiveAccount(ctx context.Context, in *pb.GetMyUserActiveAccountsRequest) (*pb.GetMyUserActiveAccountsResponse, error) {
	theUser, err := v2.GetMyUser(primitive.NilObjectID, in.Eid, "", "")
	if err != nil {
		return nil, err
	}

	activeAccount, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		return nil, err
	}

	pbActiveAccount := mapper.MapAccountSchemaToProto(activeAccount)

	return &pb.GetMyUserActiveAccountsResponse{
		ActiveAccount: pbActiveAccount,
	}, nil
}
