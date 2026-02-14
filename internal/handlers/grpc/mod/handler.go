package mod

import (
	"context"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	pbModels "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated/models"
)

type Handler struct {
	pb.UnimplementedAgentModConfigServiceServer
}

func (s *Handler) GetModConfig(ctx context.Context, _ *pbModels.SSMEmpty) (*pb.AgentModConfigResponse, error) {

	// Extract API key
	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	agentModConfig, err := agent.GetAgentModConfig(*apiKey)
	if err != nil {
		logger.GetErrorLogger().Println("Invalid API key:", err)
		return nil, err
	}

	// ---- SEND INITIAL MOD LIST ----
	configData := &pbModels.ModConfig{}
	utils.StructToPBStruct(agentModConfig, configData)

	resData := &pb.AgentModConfigResponse{
		Config: configData,
	}

	return resData, nil
}

func (s *Handler) UpdateModConfig(ctx context.Context, req *pb.AgentModConfigRequest) (*pbModels.SSMEmpty, error) {
	// Extract API key
	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	updatedModConfig := &v2.AgentModConfig{}
	utils.StructToPBStruct(req.Config, updatedModConfig)

	if err := agent.UpdateAgentModConfig(*apiKey, updatedModConfig); err != nil {
		return nil, err
	}

	return &pbModels.SSMEmpty{}, nil
}
