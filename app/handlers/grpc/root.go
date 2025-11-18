package grpc

import (
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/handlers/grpc/config"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/handlers/grpc/logs"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/handlers/grpc/mod"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/handlers/grpc/state"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/handlers/grpc/task"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/logger"
	"google.golang.org/grpc"

	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
)

func InitgRPCHandlers(grpcServer *grpc.Server) {
	logger.GetDebugLogger().Println("Initalizing all gRPC services")
	pb.RegisterAgentLogServiceServer(grpcServer, &logs.Handler{})
	pb.RegisterAgentConfigServiceServer(grpcServer, &config.Handler{})
	pb.RegisterAgentStateServiceServer(grpcServer, &state.Handler{})
	pb.RegisterAgentTaskServiceServer(grpcServer, &task.Handler{})
	pb.RegisterAgentModConfigServiceServer(grpcServer, &mod.Handler{})
	logger.GetDebugLogger().Println("Initalized all gRPC services")
}

func ShutdownGRPCServices() {
	logger.GetDebugLogger().Println("Shutting down all gRPC handlers")
	state.ShutdownAgentStateHandler()
	logs.ShutdownAgentLogHandler()
	logger.GetDebugLogger().Println("Shutdown all gRPC handlers")
}
