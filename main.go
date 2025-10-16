package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/handlers"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/mrhid6/go-mongoose/mongoose"
)

func main() {
	if err := config.InitConfig(); err != nil {
		utils.CheckError(err)
	}

	godotenv.Load(".env", ".env.local")
	if err := repositories.InitDBRepository(); err != nil {
		panic(err)
	}

	ConfigData, err := config.GetConfigData()
	utils.CheckError(err)

	dbConnection := mongoose.DBConnection{
		Host:     ConfigData.Database.Host,
		Port:     ConfigData.Database.Port,
		Database: ConfigData.Database.DB,
		User:     ConfigData.Database.User,
		Password: ConfigData.Database.Pass,
	}

	mongoose.InitiateDB(dbConnection)

	if err := mongoose.TestConnection(); err != nil {
		panic(err)
	}

	err = repositories.CreateSSMBucket()
	utils.CheckError(err)

	services.InitAllServices()

	PrintConnectionString(dbConnection)

	MainRouter := gin.Default()
	MainRouter.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"success": false, "error": "Page not found"})
	})
	apiGroup := MainRouter.Group("api")

	handlers.NewV1Handler(apiGroup)
	handlers.NewV2Handler(apiGroup)

	srv := &http.Server{
		Addr:    ConfigData.HTTPBind,
		Handler: MainRouter,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	wait := gracefulShutdown(context.Background(), 30*time.Second, map[string]operation{
		"gin": func(ctx context.Context) error {
			return srv.Shutdown(ctx)
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
