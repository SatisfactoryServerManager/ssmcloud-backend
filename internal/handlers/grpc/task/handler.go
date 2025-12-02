package task

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
)

type Handler struct {
	pb.UnimplementedAgentTaskServiceServer
}

func (s *Handler) GetAgentTasks(ctx context.Context, in *pb.Empty) (*pb.AgentTaskList, error) {
	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	theAgent, err := agent.GetAgentByAPIKey(*apiKey)
	if err != nil {
		return nil, fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	result := &pb.AgentTaskList{}
	for _, t := range theAgent.Tasks {
		dataStr, _ := json.Marshal(t.Data)
		result.Tasks = append(result.Tasks, &pb.AgentTask{
			Id:        t.ID.Hex(),
			Action:    t.Action,
			Data:      string(dataStr),
			Completed: t.Completed,
			Retries:   int32(t.Retries),
		})
	}

	return result, nil
}

func (s *Handler) MarkAgentTaskCompleted(ctx context.Context, in *pb.AgentTaskCompletedRequest) (*pb.Empty, error) {

	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := agent.MarkAgentTaskCompleted(*apiKey, in.Id); err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}

func (s *Handler) MarkAgentTaskFailed(ctx context.Context, in *pb.AgentTaskFailedRequest) (*pb.Empty, error) {

	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := agent.MarkAgentTaskFailed(*apiKey, in.Id); err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}
