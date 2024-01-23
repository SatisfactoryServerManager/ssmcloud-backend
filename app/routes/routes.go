package routes

import "github.com/gin-gonic/gin"

type routes struct {
	Router       *gin.Engine
	MainGroup    *gin.RouterGroup
	AccountGroup *gin.RouterGroup
}

func (obj *routes) SetupAPIGroups() {
	obj.MainGroup = obj.Router.Group("api").Group("v1")
	obj.AccountGroup = obj.MainGroup.Group("account")
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
