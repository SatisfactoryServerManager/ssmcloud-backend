package app

import (
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
)

type Vector3f struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

type Middleware_Session_JWT struct {
	SessionID string `json:"sessionId"`
	AccountID string `json:"accountId"`
	UserID    string `json:"userId"`
}

type API_AccountLogin_PostData struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type API_AccountSignup_PostData struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	AccountName string `json:"accountName"`
}

type API_AccountUserTwoFACode_PostData struct {
	Token string `json:"token"`
}

type API_AccountUserApiKey_PostData struct {
	APIKey string `json:"apikey"`
}

type API_AccountAgentTask_PostData struct {
	Action string      `json:"action"`
	Data   interface{} `json:"data"`
}

type API_AccountAgentMod_PostData struct {
	ModReference string `json:"modReference"`
}

type API_AccountAgentConfig_PutData struct {
	UpdatedAgent models.Agents `json:"updatedAgent"`
}

type API_AgentStatus_PutData struct {
	Online             bool    `json:"online"`
	Installed          bool    `json:"installed"`
	Running            bool    `json:"running"`
	CPU                float64 `json:"cpu"`
	MEM                float32 `json:"mem"`
	InstalledSFVersion int64   `json:"installedSFVersion"`
	LatestSFVersion    int64   `json:"latestSFVersion"`
}

type API_AgentTaskItem_PutData struct {
	Item models.AgentTask `json:"item"`
}

type API_AgentConfig_ResData struct {
	Config       models.AgentConfig       `json:"config"`
	ServerConfig models.AgentServerConfig `json:"serverConfig"`
}

type API_AgentConfig_PutData struct {
	Version string `json:"version"`
	IP      string `json:"ip"`
}

type API_CreateAccountUser_PostData struct {
	Email string `json:"email"`
}

type API_AcceptInvite_PostData struct {
	Password string `json:"password"`
}

type API_UpdatePlayers_PostData struct {
	Players []API_UpdatePlayers_Player_PostData `json:"players"`
}

type API_UpdatePlayers_Player_PostData struct {
	Name     string          `json:"name"`
	Location models.Vector3F `json:"location"`
}

type API_UpdateBuildings_PostData struct {
	Buildings []API_UpdateBuildings_Building_PostData `json:"buildings"`
}

type API_UpdateBuildings_Building_PostData struct {
	Name        string             `json:"name"`
	Location    models.Vector3F    `json:"location"`
	Class       string             `json:"class"`
	Rotation    float32            `json:"rotation"`
	BoundingBox models.BoundingBox `json:"boundingBox"`
}

type FicsitAPI_Response_GetMods struct {
	GetMods FicsitAPI_Response_GetMods_Mods `json:"getMods"`
}

type FicsitAPI_Response_GetMods_Mods struct {
	Mods []models.Mods `json:"mods"`
}
