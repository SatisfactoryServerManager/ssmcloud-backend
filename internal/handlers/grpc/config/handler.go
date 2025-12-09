package config

import (
	"context"
	"fmt"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
)

type Handler struct {
	pb.UnimplementedAgentConfigServiceServer
}

func (s *Handler) GetAgentConfig(ctx context.Context, in *pb.Empty) (*pb.AgentConfigResponse, error) {

	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	config, err := agent.GetAgentConfig(*apiKey)
	if err != nil {
		return nil, fmt.Errorf("error getting agent config with error: %s", err.Error())
	}

	res := &pb.AgentConfigResponse{
		Config: &pb.AgentConfig{
			BackupInterval:   int32(config.Config.BackupInterval),
			BackupKeepAmount: int32(config.Config.BackupKeepAmount),
		},
		ServerConfig: &pb.AgentServerConfig{
			MaxPlayers:            int32(config.ServerConfig.MaxPlayers),
			WorkerThreads:         int32(config.ServerConfig.WorkerThreads),
			Branch:                config.ServerConfig.Branch,
			UpdateSFOnStart:       config.ServerConfig.UpdateOnStart,
			AutoRestart:           config.ServerConfig.AutoRestart,
			AutoPause:             config.ServerConfig.AutoPause,
			AutoSaveOnDisconnect:  config.ServerConfig.AutoSaveOnDisconnect,
			AutoSaveInterval:      int32(config.ServerConfig.AutoSaveInterval),
			DisableSeasonalEvents: config.ServerConfig.DisableSeasonalEvents,
		},
	}

	return res, nil
}

func (s *Handler) UpdateAgentConfigVersionIp(ctx context.Context, in *pb.UpdateAgentConfigRequest) (*pb.Empty, error) {

	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	return nil, agent.UpdateAgentConfigApi(*apiKey, in.Version, in.Ip)
}
