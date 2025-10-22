package handlers

import (
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/app/services/v2"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FrontendUserAccountAgentsHandler struct{}

func (handler *FrontendUserAccountAgentsHandler) API_GetMyAccountAgents(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	id := c.Query("_id")
	var oid primitive.ObjectID
	if id != "" {
		oid, _ = primitive.ObjectIDFromHex(id)
	}

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	agents, err := v2.GetMyUserAccountAgents(account, oid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "agents": agents})
}

func (handler *FrontendUserAccountAgentsHandler) API_CreateAgent(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	PostData := &models.CreateAgentWorkflowData{}
	if err := c.BindJSON(PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if PostData.AgentName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent name is empty", "success": false})
		c.Abort()
		return
	}

	workflowId, err := v2.CreateAgentWorkflow(account.ID, PostData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "workflow_id": workflowId})
}

func (handler *FrontendUserAccountAgentsHandler) API_DeleteAgent(c *gin.Context) {

	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	id := c.Query("_id")
	var oid primitive.ObjectID

	if id == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "id was empty", "success": false})
		c.Abort()
		return
	}

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if err := v2.DeleteAgent(account, oid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": ""})
}

func (handler *FrontendUserAccountAgentsHandler) API_UpdateAgentSettings(c *gin.Context) {

	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	PostData := &app.APIUpdateServerSettingsRequest{}
	if err := c.BindJSON(PostData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if PostData.ID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "id was empty", "success": false})
		c.Abort()
		return
	}

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if err := v2.UpdateAgentSettings(account, PostData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": ""})
}

func (handler *FrontendUserAccountAgentsHandler) API_GetAgentLog(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	id := c.Query("_id")
	logType := c.Query("type")

	if id == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "agent id is empty", "success": false})
		c.Abort()
		return
	}

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	agents, err := v2.GetMyUserAccountAgents(account, oid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if len(agents) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent was not found", "success": false})
		c.Abort()
		return
	}

	theAgent := agents[0]

	theLog, err := v2.GetAgentLog(theAgent, logType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "agentLog": theLog})
}

func (handler *FrontendUserAccountAgentsHandler) API_CreateAgentTask(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	type TaskPostData struct {
		Action  string             `json:"action"`
		AgentId primitive.ObjectID `json:"id"`
		Data    interface{}        `json:"data"`
	}

	PostData := &TaskPostData{}
	if err := c.BindJSON(PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	agents, err := v2.GetMyUserAccountAgents(account, PostData.AgentId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if len(agents) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent was not found", "success": false})
		c.Abort()
		return
	}

	theAgent := agents[0]

	if err := v2.CreateAgentTask(theAgent, PostData.Action, PostData.Data); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent was not found", "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": ""})
}

func (handler *FrontendUserAccountAgentsHandler) API_GetAgentMods(c *gin.Context) {
	agentId := c.Query("agentId")
	page := c.Query("page")
	sort := c.Query("sort")
	direction := c.Query("direction")
	search := c.Query("search")

	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	oid, err := primitive.ObjectIDFromHex(agentId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	theAccount, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	agents, err := v2.GetMyUserAccountAgents(theAccount, oid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}
	theAgent := agents[0]

	pageInt, _ := strconv.Atoi(page)
	mods, err := v2.GetMods(pageInt, sort, direction, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}
	modCount, err := v2.GetModCount()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	pages := float64(modCount) / float64(30)

	ModModel, err := repositories.GetMongoClient().GetModel("AgentModConfigSelectedMod")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	for idx := range theAgent.ModConfig.SelectedMods {
		mod := &theAgent.ModConfig.SelectedMods[idx]
		if err := ModModel.PopulateField(mod, "Mod"); err != nil {
			err = fmt.Errorf("error populating mod with error: %s", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
			c.Abort()
			return
		}
	}

	c.JSON(200, gin.H{"success": true, "mods": mods, "totalMods": modCount, "pages": int(math.Ceil(pages)), "agentModConfig": theAgent.ModConfig})

}

func (h *FrontendUserAccountAgentsHandler) API_AgentInstallMod(c *gin.Context) {

	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	var PostData app.API_AccountAgentMod_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	agents, err := v2.GetMyUserAccountAgents(account, PostData.AgentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if len(agents) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent was not found", "success": false})
		c.Abort()
		return
	}

	theAgent := agents[0]

	err = v2.UpdateMod(theAgent, PostData.ModReference)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *FrontendUserAccountAgentsHandler) API_AgentUpdateMod(c *gin.Context) {

	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	var PostData app.API_AccountAgentMod_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	agents, err := v2.GetMyUserAccountAgents(account, PostData.AgentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if len(agents) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent was not found", "success": false})
		c.Abort()
		return
	}

	theAgent := agents[0]

	err = v2.UpdateMod(theAgent, PostData.ModReference)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *FrontendUserAccountAgentsHandler) API_AgentUninstallMod(c *gin.Context) {

	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	var PostData app.API_AccountAgentMod_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	agents, err := v2.GetMyUserAccountAgents(account, PostData.AgentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if len(agents) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent was not found", "success": false})
		c.Abort()
		return
	}

	theAgent := agents[0]

	err = v2.UninstallMod(theAgent, PostData.ModReference)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func NewFrontendUserAccountAgentHandler(router *gin.RouterGroup) {

	handler := FrontendUserAccountAgentsHandler{}

	router.GET("/", handler.API_GetMyAccountAgents)
	router.POST("/", handler.API_CreateAgent)
	router.DELETE("/", handler.API_DeleteAgent)
	router.POST("/settings", handler.API_UpdateAgentSettings)
	router.GET("/log", handler.API_GetAgentLog)
	router.POST("/tasks", handler.API_CreateAgentTask)

	router.GET("/mods", handler.API_GetAgentMods)
	router.POST("/installmod", handler.API_AgentUpdateMod)
	router.POST("/uninstallmod", handler.API_AgentUninstallMod)
}
