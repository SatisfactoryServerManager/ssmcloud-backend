package config

import (
	"context"
	"fmt"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	pbModels "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated/models"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/utils/mapper"
)

type Handler struct {
	pb.UnimplementedAgentConfigServiceServer
}

func (s *Handler) GetAgentConfig(ctx context.Context, in *pbModels.SSMEmpty) (*pb.AgentConfigResponse, error) {

	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	config, err := agent.GetAgentConfig(*apiKey)
	if err != nil {
		return nil, fmt.Errorf("error getting agent config with error: %s", err.Error())
	}

	res := &pb.AgentConfigResponse{
		Config:       mapper.MapAgentConfigToProto(&config.Config),
		ServerConfig: mapper.MapAgentServerConfigToProto(&config.ServerConfig),
	}

	return res, nil
}

func (s *Handler) UpdateAgentConfigVersionIp(ctx context.Context, in *pb.UpdateAgentConfigRequest) (*pbModels.SSMEmpty, error) {

	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		return &pbModels.SSMEmpty{}, err
	}

	return &pbModels.SSMEmpty{}, agent.UpdateAgentConfigApi(*apiKey, in.Version, in.Ip)
}
