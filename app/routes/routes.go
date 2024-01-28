package routes

import (
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/routes/account"
	"github.com/gin-gonic/gin"
)

type routes struct {
	Router    *gin.Engine
	MainGroup *gin.RouterGroup

	// Account Groups e.g. anything that can be access through the dashboard
	AccountGroup      *gin.RouterGroup
	AccountUsersGroup *gin.RouterGroup
	AccountAgentGroup *gin.RouterGroup

	// Agent Groups e.g. anything that the agent can access

	// Mod Groups e.g. anything that the SSM mod can access
}

func (obj *routes) SetupAPIGroups() {
	obj.MainGroup = obj.Router.Group("api").Group("v1")

	obj.AccountGroup = obj.MainGroup.Group("account")
	obj.AccountUsersGroup = obj.AccountGroup.Group("users")
	obj.AccountAgentGroup = obj.AccountGroup.Group("agents")
}

func (obj *routes) AddV1Routes() {
	obj.MainGroup.GET("/ping", API_Ping)

	obj.AddAccountRoutes()
	obj.AddAccountUsersRoutes()
	obj.AddAccountAgentRoutes()
}

func (obj *routes) AddAccountRoutes() {
	obj.AccountGroup.POST("/login", account.API_AccountLogin)
	obj.AccountGroup.POST("/signup", account.API_AccountSignUp)

	obj.AccountGroup.Use(Middleware_DecodeJWT())
	obj.AccountGroup.Use(Middleware_VerifySession())

	obj.AccountGroup.GET("/session", account.API_AccountSession)

}
func (obj *routes) AddAccountUsersRoutes() {

	obj.AccountUsersGroup.Use(Middleware_DecodeJWT())
	obj.AccountUsersGroup.Use(Middleware_VerifySession())

	obj.AccountUsersGroup.GET("/", account.API_GetAllUsers)
}

func (obj *routes) AddAccountAgentRoutes() {

	obj.AccountAgentGroup.Use(Middleware_DecodeJWT())
	obj.AccountAgentGroup.Use(Middleware_VerifySession())

	obj.AccountAgentGroup.GET("/", account.API_GetAllAgents)
	obj.AccountAgentGroup.POST("/", account.API_CreateNewAgent)
}

var (
	Routes routes
)

func InitRoutes() {
	Routes = routes{}
	Routes.Router = gin.Default()
	Routes.SetupAPIGroups()

	Routes.AddV1Routes()
}
