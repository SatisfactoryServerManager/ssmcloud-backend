package routes

import (
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/routes/account"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/routes/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/routes/ssmmod"
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
	AgentGroup *gin.RouterGroup

	// Mod Groups e.g. anything that the SSM mod can access
	SSMModGroup *gin.RouterGroup
}

func (obj *routes) SetupAPIGroups() {
	obj.MainGroup = obj.Router.Group("api").Group("v1")

	obj.AccountGroup = obj.MainGroup.Group("account")
	obj.AccountUsersGroup = obj.AccountGroup.Group("users")
	obj.AccountAgentGroup = obj.AccountGroup.Group("agents")

	obj.AgentGroup = obj.MainGroup.Group("agent")

	obj.SSMModGroup = obj.MainGroup.Group("ssmmod")

}

func (obj *routes) AddV1Routes() {
	obj.MainGroup.GET("/ping", API_Ping)
	obj.MainGroup.GET("/mods", API_Mods)

	obj.AddAccountRoutes()
	obj.AddAccountUsersRoutes()
	obj.AddAccountAgentRoutes()

	obj.AddAgentRoutes()

	obj.AddSSMModRoutes()
}

func (obj *routes) AddAccountRoutes() {
	obj.AccountGroup.POST("/login", account.API_AccountLogin)
	obj.AccountGroup.POST("/signup", account.API_AccountSignUp)

	obj.AccountGroup.Use(Middleware_DecodeJWT())
	obj.AccountGroup.Use(Middleware_VerifySession())

	obj.AccountGroup.GET("/", account.API_GetAccount)
	obj.AccountGroup.GET("/session", account.API_AccountSession)
	obj.AccountGroup.GET("/audit", account.API_GetAccountAudit)

}
func (obj *routes) AddAccountUsersRoutes() {

	obj.AccountUsersGroup.GET("/byinvitecode/:invitecode", account.API_GetUserByInviteCode)
	obj.AccountUsersGroup.POST("/acceptinvite/:invitecode", account.API_AcceptUserInvite)

	obj.AccountUsersGroup.Use(Middleware_DecodeJWT())
	obj.AccountUsersGroup.Use(Middleware_VerifySession())

	obj.AccountUsersGroup.GET("/", account.API_GetAllUsers)
	obj.AccountUsersGroup.POST("/", account.API_CreateAccountUser)
	obj.AccountUsersGroup.GET("/me", account.API_GetMyUser)
	obj.AccountUsersGroup.POST("/me/twofa/generate", account.API_GenerateUserTwoFASecret)
	obj.AccountUsersGroup.POST("/me/twofa/validate", account.API_ValidateUserTwoFACode)
}

func (obj *routes) AddAccountAgentRoutes() {

	obj.AccountAgentGroup.GET("/:agentid/mapdata", account.API_GetAgentMapData)

	obj.AccountAgentGroup.Use(Middleware_DecodeJWT())
	obj.AccountAgentGroup.Use(Middleware_VerifySession())

	obj.AccountAgentGroup.GET("/", account.API_GetAllAgents)
	obj.AccountAgentGroup.POST("/", account.API_CreateNewAgent)

	obj.AccountAgentGroup.GET("/:agentid", account.API_GetAgentByID)
	obj.AccountAgentGroup.DELETE("/:agentid", account.API_DeleteAgent)

	obj.AccountAgentGroup.PUT("/:agentid/configs", account.API_UpdateAgentConfigs)

	obj.AccountAgentGroup.GET("/:agentid/logs", account.API_GetAgentLogs)

	obj.AccountAgentGroup.GET("/:agentid/tasks", account.API_GetAgentTasks)
	obj.AccountAgentGroup.POST("/:agentid/tasks", account.API_NewAgentTask)

	obj.AccountAgentGroup.POST("/:agentid/mods/install", account.API_AgentInstallMod)
	obj.AccountAgentGroup.POST("/:agentid/mods/update", account.API_AgentUpdateMod)
	obj.AccountAgentGroup.POST("/:agentid/mods/uninstall", account.API_AgentUninstallMod)

	obj.AccountAgentGroup.GET("/:agentid/download/backup/:uuid", account.API_DownloadAgentBackup)
	obj.AccountAgentGroup.GET("/:agentid/download/save/:uuid", account.API_DownloadAgentSave)
	obj.AccountAgentGroup.GET("/:agentid/download/log/:type", account.API_DownloadAgentLog)

	uploadGroup := obj.AccountAgentGroup.Group("upload")
	uploadGroup.Use(Middleware_UploadFile())

	uploadGroup.POST("/:agentid/save", account.API_UploadAgentSave)
}

func (obj *routes) AddAgentRoutes() {
	obj.AgentGroup.Use(Middleware_AgentAPIKey())

	obj.AgentGroup.PUT("/status", agent.API_UpdateAgentStatus)

	uploadGroup := obj.AgentGroup.Group("upload")
	uploadGroup.Use(Middleware_UploadFile())

	uploadGroup.POST("/save", agent.API_UploadAgentSave)
	uploadGroup.POST("/backup", agent.API_UploadAgentBackup)
	uploadGroup.POST("/log", agent.API_UploadAgentLog)

	obj.AgentGroup.GET("/modconfig", agent.API_GetModConfig)
	obj.AgentGroup.PUT("/modconfig", agent.API_UpdateModConfig)

	obj.AgentGroup.GET("/tasks", agent.API_GetAgentTasks)
	obj.AgentGroup.PUT("/tasks/:taskid", agent.API_UpdateTaskItem)

	obj.AgentGroup.GET("/config", agent.API_GetAgentConfig)
	obj.AgentGroup.PUT("/config", agent.API_UpdateAgentConfig)

	obj.AgentGroup.GET("/saves/download/:filename", agent.API_DownloadAgentSave)
}

func (obj *routes) AddSSMModRoutes() {
	obj.SSMModGroup.Use(Middleware_AgentAPIKey())

	obj.SSMModGroup.POST("/players", ssmmod.API_UpdatePlayers)
	obj.SSMModGroup.POST("/buildings", ssmmod.API_UpdateBuildings)

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
