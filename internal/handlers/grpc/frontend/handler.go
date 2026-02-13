package frontend

import (
	"context"

	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/v2"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
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
