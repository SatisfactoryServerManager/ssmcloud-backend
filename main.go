package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/handlers"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/mrhid6/go-mongoose/mongoose"
	"google.golang.org/grpc"
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

	handlers.NewV1Handler(apiGroup)
	handlers.NewV2Handler(apiGroup)

	httpBind := ":3000"
	if os.Getenv("HOST_PORT") != "" {
		httpBind = os.Getenv("HOST_PORT")
	}

	grpcBind := ":8443"
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
			log.Fatalf("failed to listen: %v", err)
		}

		services.InitGRPCServices(grpcServer)

		log.Printf("grpc server listening at %v", lis.Addr())

		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	wait := gracefulShutdown(context.Background(), 30*time.Second, map[string]operation{
		"gin": func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
		"grpc": func(ctx context.Context) error {
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
