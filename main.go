package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/routes"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
	"github.com/mrhid6/go-mongoose/mongoose"
)

func main() {
	if err := config.InitConfig(); err != nil {
		utils.CheckError(err)
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

	PrintConnectionString(dbConnection)

	routes.InitRoutes()

	srv := &http.Server{
		Addr:    ConfigData.HTTPBind,
		Handler: routes.Routes.Router,
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
	})

	<-wait
}

func PrintConnectionString(dbConnection mongoose.DBConnection) {
	fmt.Printf("mongodb connection: %v\n", dbConnection)
}
