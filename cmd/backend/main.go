package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/cleanup"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/config"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/mrhid6/go-mongoose/mongoose"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/handlers/api"
	grpcHandler "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/handlers/grpc"
)

func main() {
	if err := config.InitConfig(); err != nil {
		utils.CheckError(err)
	}

	godotenv.Load(".env", ".env.local")

	if err := repositories.InitDBRepository(); err != nil {
		panic(err)
	}

	err := repositories.CreateSSMBucket()
	utils.CheckError(err)

	services.InitAllServices()

	MainRouter := gin.Default()
	MainRouter.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"success": false, "error": "Page not found"})
	})
	apiGroup := MainRouter.Group("api")

	api.NewV1Handler(apiGroup)
	api.NewV2Handler(apiGroup)

	httpBind := ":3000"
	if os.Getenv("HOST_PORT") != "" {
		httpBind = os.Getenv("HOST_PORT")
	}

	grpcBind := ":50051"
	if os.Getenv("GRPC_PORT") != "" {
		grpcBind = os.Getenv("GRPC_PORT")
	}

	srv := &http.Server{
		Addr:    httpBind,
		Handler: MainRouter,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	grpcServer := grpc.NewServer()

	go func() {
		lis, err := net.Listen("tcp", grpcBind)
		if err != nil {
			logger.GetErrorLogger().Printf("failed to listen: %v", err)
			panic(err)
		}

		grpcHandler.InitgRPCHandlers(grpcServer)

		logger.GetInfoLogger().Printf("gRPC server listening at %v", lis.Addr())

		reflection.Register(grpcServer)

		if err := grpcServer.Serve(lis); err != nil {
			logger.GetErrorLogger().Printf("failed to serve: %v", err)
			panic(err)
		}
	}()

	wait := cleanup.GracefulShutdown(context.Background(), 30*time.Second, map[string]cleanup.CleanupOperation{
		"gin": func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
		"grpc": func(ctx context.Context) error {
			grpcHandler.ShutdownGRPCServices()
			grpcServer.GracefulStop()
			return nil
		},
		"services": func(ctx context.Context) error {
			return services.ShutdownAllServices()
		},
	})

	<-wait
}

func PrintConnectionString(dbConnection mongoose.DBConnection) {
	fmt.Printf("mongodb connection: %v\n", dbConnection)
}
