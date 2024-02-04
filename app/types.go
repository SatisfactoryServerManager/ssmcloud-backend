package app

import (
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	resolver "github.com/satisfactorymodding/ficsit-resolver"
)

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
type API_AccountCreateAgent_PostData struct {
	AgentName string `json:"agentName"`
	Port      int    `json:"port"`
	Memory    int64  `json:"memory"`
}

type API_AccountUserTwoFACode_PostData struct {
	Token string `json:"token"`
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

type FicsitAPI_Response_GetMods struct {
	GetMods FicsitAPI_Response_GetMods_Mods `json:"getMods"`
}

type FicsitAPI_Response_GetMods_Mods struct {
	Mods []models.Mods `json:"mods"`
}

type FicsitAPI_Response_GetSMLVersions struct {
	GetSMLVersions FicsitAPI_Response_GetSMLVersions_SMLVersions `json:"getSMLVersions"`
}

type FicsitAPI_Response_GetSMLVersions_SMLVersions struct {
	SMLVersions []resolver.SMLVersion `json:"sml_versions"`
}
