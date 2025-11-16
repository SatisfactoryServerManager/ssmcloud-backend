package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type GRPCAgentServiceServer struct {
	pb.UnimplementedAgentServiceServer
}

var (
	UpdateAgentStateCancel context.CancelFunc
)

func getAPIKeyFromContext(ctx context.Context) (*string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	apiKeys := md["x-api-key"]
	if len(apiKeys) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "missing API key")
	}

	apiKey := &apiKeys[0]

	return apiKey, nil
}

func (s *GRPCAgentServiceServer) GetAgentConfig(ctx context.Context, in *pb.Empty) (*pb.AgentConfigResponse, error) {

	apiKey, err := getAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	config, err := GetAgentConfig(*apiKey)
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

func (s *GRPCAgentServiceServer) UpdateAgentConfigVersionIp(ctx context.Context, in *pb.AgentConfigRequest) (*pb.Empty, error) {

	apiKey, err := getAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	return nil, UpdateAgentConfigApi(*apiKey, in.Version, in.Ip)
}

func (s *GRPCAgentServiceServer) UpdateAgentStateStream(stream pb.AgentService_UpdateAgentStateStreamServer) error {

	ctx, cancel := context.WithCancel(stream.Context())
	UpdateAgentStateCancel = cancel
	defer cancel()

	msgChan := make(chan *pb.AgentStateRequest)
	errChan := make(chan error)

	// Recv goroutine (since Recv() blocks forever)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				errChan <- err
				return
			}
			msgChan <- msg
		}
	}()

	apiKey, err := getAPIKeyFromContext(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			// **Instant** exit when Cancel() is called
			return stream.SendAndClose(&pb.Empty{})

		case err := <-errChan:
			if err == io.EOF {
				return stream.SendAndClose(&pb.Empty{})
			}
			return err

		case msg := <-msgChan:
			if err := UpdateAgentStatus(*apiKey, msg.Online, msg.Installed, msg.Running, float64(msg.Cpu), msg.Ram, msg.InstalledSFVersion, msg.LatestSFVersion); err != nil {
				return err
			}
			fmt.Printf("state: %+v\n", msg)
		}
	}
}

func (s *GRPCAgentServiceServer) UpdateAgentState(ctx context.Context, msg *pb.AgentStateRequest) (*pb.Empty, error) {
	apiKey, err := getAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := UpdateAgentStatus(*apiKey, msg.Online, msg.Installed, msg.Running, float64(msg.Cpu), msg.Ram, msg.InstalledSFVersion, msg.LatestSFVersion); err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}

func (s *GRPCAgentServiceServer) GetAgentTasks(ctx context.Context, in *pb.Empty) (*pb.AgentTaskList, error) {
	apiKey, err := getAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	theAgent, err := GetAgentByAPIKey(*apiKey)
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

func (s *GRPCAgentServiceServer) MarkAgentTaskCompleted(ctx context.Context, in *pb.AgentTaskCompletedRequest) (*pb.Empty, error) {

	apiKey, err := getAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := MarkAgentTaskCompleted(*apiKey, in.Id); err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}

func (s *GRPCAgentServiceServer) MarkAgentTaskFailed(ctx context.Context, in *pb.AgentTaskFailedRequest) (*pb.Empty, error) {

	apiKey, err := getAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := MarkAgentTaskFailed(*apiKey, in.Id); err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}

func InitGRPCServices(grpcServer *grpc.Server) {
	log.Println("Init GRPC Services")
	pb.RegisterAgentServiceServer(grpcServer, &GRPCAgentServiceServer{})
}

func ShutdownGRPCServices() {
	UpdateAgentStateCancel()
}
