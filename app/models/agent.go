package models

import (
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Agents struct {
	ID        primitive.ObjectID `json:"_id" bson:"_id"`
	AgentName string             `json:"agentName" bson:"agentName"`
	APIKey    string             `json:"apiKey" bson:"apiKey"`
	Status    AgentStatus        `json:"status" bson:"status"`

	Config       AgentConfig       `json:"config" bson:"config"`
	ServerConfig AgentServerConfig `json:"serverConfig" bson:"serverConfig"`

	MapData AgentMapData `json:"mapData" bson:"mapData"`

	Saves   []AgentSave   `json:"saves" bson:"saves"`
	Backups []AgentBackup `json:"backups" bson:"backups"`

	Tasks []AgentTask `json:"tasks" bson:"tasks"`

	Logs       primitive.A `json:"-" bson:"logs" mson:"collection=agentlogs"`
	LogObjects []AgentLogs `json:"logs" bson:"-"`

	ModConfig AgentModConfig `json:"modConfig" bson:"modConfig"`

	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

type AgentStatus struct {
	Online    bool `json:"online" bson:"online"`
	Running   bool `json:"running" bson:"running"`
	Installed bool `json:"installed" bson:"installed"`

	CPU float64 `json:"cpu" bson:"cpu"`
	RAM float64 `json:"ram" bson:"ram"`

	LastCommDate time.Time `json:"lastCommDate" bson:"lastCommDate"`

	InstalledSFVersion int64 `json:"installedSFVersion"`
	LatestSFVersion    int64 `json:"latestSFVersion"`
}

type AgentConfig struct {
	Version          string  `json:"version" bson:"version"`
	Port             int     `json:"port" bson:"port"`
	Memory           int64   `json:"memory" bson:"memory"`
	IP               string  `json:"ip" bson:"ip"`
	BackupKeepAmount int     `json:"backupKeepAmount" bson:"backupKeepAmount"`
	BackupInterval   float32 `json:"backupInterval" bson:"backupInterval"`
}

type AgentServerConfig struct {
	UpdateOnStart bool   `json:"updateOnStart" bson:"updateOnStart"`
	Branch        string `json:"branch" bson:"branch"`
	WorkerThreads int    `json:"workerThreads" bson:"workerThreads"`

	AutoRestart bool `json:"autoRestart" bson:"autoRestart"`

	// Settings for Server Ini files
	MaxPlayers            int  `json:"maxPlayers" bson:"maxPlayers"`
	AutoPause             bool `json:"autoPause" bson:"autoPause"`
	AutoSaveOnDisconnect  bool `json:"autoSaveOnDisconnect" bson:"autoSaveOnDisconnect"`
	AutoSaveInterval      int  `json:"autoSaveInterval" bson:"autoSaveInterval"`
	DisableSeasonalEvents bool `json:"disableSeasonalEvents" bson:"disableSeasonalEvents"`
}

// Map Data

type AgentMapData struct {
	Players   []AgentMapDataPlayer   `json:"players" bson:"players"`
	Buildings []AgentMapDataBuilding `json:"buildings" bson:"buildings"`
}

type AgentMapDataPlayer struct {
	Username string   `json:"username" bson:"username"`
	Location Vector3F `json:"location" bson:"location"`
}

type AgentMapDataBuilding struct {
	Type     string   `json:"type" bson:"type"`
	Location Vector3F `json:"location" bson:"location"`
	Rotation float32  `json:"rotation" bson:"rotation"`
}

// Save Data

type AgentSave struct {
	UUID        string `json:"uuid" bson:"uuid"`
	FileName    string `json:"fileName" bson:"fileName"`
	SessionName string `json:"sessionName" bson:"sessionName"`
	Size        int64  `json:"size" bson:"size"`

	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

// Backup Data

type AgentBackup struct {
	UUID     string `json:"uuid" bson:"uuid"`
	FileName string `json:"fileName" bson:"fileName"`
	Size     int64  `json:"size" bson:"size"`

	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

// Mod Data

type AgentModConfig struct {
	LatestSMLVersion    string                      `json:"lastestSMLVersion" bson:"lastestSMLVersion"`
	InstalledSMLVersion string                      `json:"installedSMLVersion" bson:"installedSMLVersion"`
	SelectedMods        []AgentModConfigSelectedMod `json:"selectedMods" bson:"selectedMods"`
}

type AgentModConfigSelectedMod struct {
	Mod              primitive.ObjectID `json:"-" bson:"mod" mson:"collection=mods"`
	ModObject        Mods               `json:"mod" bson:"-"`
	DesiredVersion   string             `json:"desiredVersion" bson:"desiredVersion"`
	InstalledVersion string             `json:"installedVersion" bson:"installedVersion"`
	Installed        bool               `json:"installed" bson:"installed"`
	NeedsUpdate      bool               `json:"needsUpdate" bson:"needsUpdate"`
	Config           string             `json:"config" bson:"config"`
}

// Task Data

type AgentTask struct {
	ID        primitive.ObjectID `json:"_id" bson:"_id"`
	Action    string             `json:"action" bson:"action"`
	Data      interface{}        `json:"data" bson:"data"`
	Completed bool               `json:"completed" bson:"completed"`
	Retries   int                `json:"retries" bson:"retries"`
}

type AgentLogs struct {
	ID        primitive.ObjectID `json:"_id" bson:"_id"`
	FileName  string             `json:"fileName" bson:"fileName"`
	Type      string             `json:"type" bson:"type"`
	Snippet   string             `json:"snippet" bson:"snippet"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt" bson:"updatedAt"`
}

func (obj *Agents) PopulateModConfig() {

	for idx := range obj.ModConfig.SelectedMods {
		selectedMod := &obj.ModConfig.SelectedMods[idx]
		selectedMod.PopulateMod()
	}
}

func (obj *Agents) PopulateLogs() error {
	err := mongoose.PopulateObjectArray(obj, "Logs", &obj.LogObjects)

	if err != nil {
		return err
	}

	if obj.LogObjects == nil {
		obj.LogObjects = make([]AgentLogs, 0)
	}

	return nil
}

func (obj *Agents) PurgeTasks() error {
	newTaskList := make([]AgentTask, 0)
	for _, task := range obj.Tasks {

		if task.Completed {
			continue
		}

		newTaskList = append(newTaskList, task)
	}

	dbUpdate := bson.D{{"$set", bson.D{
		{"tasks", newTaskList},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(*obj, dbUpdate); err != nil {
		return err
	}

	return nil
}

func (obj *AgentModConfigSelectedMod) PopulateMod() {
	mongoose.PopulateObject(obj, "Mod", &obj.ModObject)
}

func NewAgent(agentName string, port int, memory int64) Agents {

	apiKey := "API-AGT-" + strings.ToUpper(utils.RandStringBytes(24))

	newAgent := Agents{
		ID:        primitive.NewObjectID(),
		AgentName: agentName,
		APIKey:    apiKey,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	newAgent.Config.Port = port
	newAgent.Config.Memory = memory

	newAgent.Config.BackupKeepAmount = 24
	newAgent.Config.BackupInterval = 1.0

	newAgent.ServerConfig.MaxPlayers = 4
	newAgent.ServerConfig.WorkerThreads = 20
	newAgent.ServerConfig.Branch = "public"
	newAgent.ServerConfig.UpdateOnStart = true
	newAgent.ServerConfig.AutoSaveInterval = 300
	newAgent.ServerConfig.AutoSaveOnDisconnect = true

	newAgent.MapData.Players = make([]AgentMapDataPlayer, 0)
	newAgent.MapData.Buildings = make([]AgentMapDataBuilding, 0)

	newAgent.Saves = make([]AgentSave, 0)
	newAgent.Backups = make([]AgentBackup, 0)
	newAgent.Tasks = make([]AgentTask, 0)

	newAgent.ModConfig.InstalledSMLVersion = "0.0.0"
	newAgent.ModConfig.LatestSMLVersion = "0.0.0"
	newAgent.ModConfig.SelectedMods = make([]AgentModConfigSelectedMod, 0)

	return newAgent
}

func NewAgentTask(action string, data interface{}) AgentTask {
	return AgentTask{
		ID:     primitive.NewObjectID(),
		Action: action,
		Data:   data,
	}
}
