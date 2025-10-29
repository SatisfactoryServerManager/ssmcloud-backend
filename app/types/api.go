package types

import (
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type API_AccountAgentMod_PostData struct {
	ModReference string             `json:"modReference"`
	AgentID      primitive.ObjectID `json:"agentId"`
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
	Item modelsv2.AgentTask `json:"item"`
}

type API_AgentConfig_ResData struct {
	Config       modelsv2.AgentConfig       `json:"config"`
	ServerConfig modelsv2.AgentServerConfig `json:"serverConfig"`
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
	Mods []models.ModSchema `json:"mods"`
}

type APIUpdateServerSettings struct {
	ConfigSetting        string  `form:"_ConfigSetting" json:"configSetting"`
	UpdateOnStart        string  `form:"inp_updateonstart" json:"updateOnStart"`
	AutoRestart          string  `form:"inp_autorestart" json:"autoRestart"`
	AutoPause            string  `form:"inp_autoPause" json:"autoPause"`
	AutoSaveOnDisconnect string  `form:"inp_autoSaveOnDisconnect" json:"autoSaveOnDisconnect"`
	AutoSaveInterval     int     `form:"inp_autoSaveInterval" json:"autoSaveInterval"`
	SeasonalEvents       string  `form:"inp_seasonalEvents" json:"seasonalEvents"`
	MaxPlayers           int     `form:"inp_maxplayers" json:"maxPlayers"`
	WorkerThreads        int     `form:"inp_workerthreads" json:"workerThreads"`
	Branch               string  `form:"inp_sfbranch" json:"branch"`
	BackupInterval       float32 `form:"inp_backupinterval" json:"backupInterval"`
	BackupKeep           int     `form:"inp_backupkeep" json:"backupKeep"`
	ModReference         string  `form:"inp_mod_ref" json:"modReference"`
	ModConfig            string  `form:"inp_modConfig" json:"modConfig"`
}

type APIUpdateServerSettingsRequest struct {
	APIUpdateServerSettings
	ID string `json:"agentId"`
}
