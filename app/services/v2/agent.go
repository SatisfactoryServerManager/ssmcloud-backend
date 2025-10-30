package v2

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/types"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	resolver "github.com/satisfactorymodding/ficsit-resolver"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetMyUserAccountAgents(theAccount *modelsv2.AccountSchema, agentId primitive.ObjectID) ([]*modelsv2.AgentSchema, error) {

	if theAccount == nil {
		emptyArray := make([]*modelsv2.AgentSchema, 0)
		return emptyArray, nil
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return nil, err
	}

	if err := AccountModel.PopulateField(theAccount, "Agents"); err != nil {
		return nil, fmt.Errorf("error populating agents with error: %s", err.Error())
	}

	if !agentId.IsZero() {
		singleAgentArray := make([]*modelsv2.AgentSchema, 0)
		for idx := range theAccount.Agents {
			agent := &theAccount.Agents[idx]
			if agent.ID.Hex() == agentId.Hex() {
				singleAgentArray = append(singleAgentArray, agent)
			}
		}

		if len(singleAgentArray) == 0 {
			return nil, errors.New("error cant find agent with id")
		}
		return singleAgentArray, nil
	}

	resArray := make([]*modelsv2.AgentSchema, 0)
	for idx := range theAccount.Agents {
		agent := &theAccount.Agents[idx]
		resArray = append(resArray, agent)
	}
	return resArray, nil
}

func CreateAgentWorkflow(accountId primitive.ObjectID, PostData *modelsv2.CreateAgentWorkflowData) (string, error) {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return "", fmt.Errorf("error getting account model with error: %s", err.Error())
	}

	WorkflowModel, err := repositories.GetMongoClient().GetModel("Workflow")
	if err != nil {
		return "", fmt.Errorf("error getting workflow model with error: %s", err.Error())
	}

	theAccount := &modelsv2.AccountSchema{}

	if err := AccountModel.FindOneById(theAccount, accountId); err != nil {
		return "", fmt.Errorf("error finding account from session with error: %s", err.Error())
	}

	if err := AccountModel.PopulateField(theAccount, "Agents"); err != nil {
		return "", fmt.Errorf("error populating account agents with error: %s", err.Error())
	}

	for _, agent := range theAccount.Agents {
		if agent.AgentName == PostData.AgentName {
			return "", fmt.Errorf("error agent with same name %s already exists on your account", PostData.AgentName)
		}
	}

	PostData.AccountId = theAccount.ID

	createAgentAction := modelsv2.WorkflowAction{
		Type: modelsv2.WorkflowActionType_CreateAgent,
	}

	waitForOnlineAction := modelsv2.WorkflowAction{
		Type: modelsv2.WorkflowActionType_WaitForOnline,
	}

	installServerAction := modelsv2.WorkflowAction{
		Type: modelsv2.WorkflowActionType_InstallServer,
	}

	waitForInstalledAction := modelsv2.WorkflowAction{
		Type: modelsv2.WorkflowActionType_WaitForInstalled,
	}

	startServerAction := modelsv2.WorkflowAction{
		Type: modelsv2.WorkflowActionType_StartServer,
	}

	waitForRunningAction := modelsv2.WorkflowAction{
		Type: modelsv2.WorkflowActionType_WaitForRunning,
	}

	claimServerAction := modelsv2.WorkflowAction{
		Type: modelsv2.WorkflowActionType_ClaimServer,
	}

	workflow := modelsv2.WorkflowSchema{
		ID:   primitive.NewObjectID(),
		Type: modelsv2.WorkflowType_CreateAgent,
		Data: PostData,
		Actions: []modelsv2.WorkflowAction{
			createAgentAction,
			waitForOnlineAction,
			installServerAction,
			waitForInstalledAction,
			startServerAction,
			waitForRunningAction,
			claimServerAction,
		},
	}

	if err := WorkflowModel.Create(workflow); err != nil {
		return "", err
	}

	return workflow.ID.Hex(), nil
}

func DeleteAgent(theAccount *modelsv2.AccountSchema, agentId primitive.ObjectID) error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	if err := AccountModel.PopulateField(theAccount, "Agents"); err != nil {
		return fmt.Errorf("error populating agents with error: %s", err.Error())
	}

	var theAgent *modelsv2.AgentSchema
	for idx := range theAccount.Agents {
		agent := &theAccount.Agents[idx]
		if agent.ID.Hex() == agentId.Hex() {
			theAgent = agent
		}
	}

	if theAgent == nil {
		return errors.New("error finding agent with that id")
	}

	newAgents := make(primitive.A, 0)
	for _, agent := range theAccount.Agents {
		if agent.ID.Hex() != agentId.Hex() {
			newAgents = append(newAgents, agent.ID)
		}
	}

	theAccount.AgentIds = newAgents

	if err := AccountModel.UpdateData(theAccount, bson.M{"agents": theAccount.AgentIds}); err != nil {
		return fmt.Errorf("error removing agent from account with error: %s", err.Error())
	}

	if err := AgentModel.DeleteById(agentId); err != nil {
		return fmt.Errorf("error deleting agent from db with error: %s", err.Error())
	}

	if err := AddAccountAudit(theAccount,
		modelsv2.AuditType_AgentRemoveFromAccount,
		fmt.Sprintf("Agent (%s) was removed from the account", theAgent.AgentName),
	); err != nil {
		return err
	}

	if err := AddIntegrationEvent(theAccount, modelsv2.IntegrationEventTypeAgentRemoved, models.EventDataAgent{
		AgentName: theAgent.AgentName,
	}); err != nil {
		return err
	}

	return nil
}

func UpdateAgentSettings(theAccount *modelsv2.AccountSchema, PostData *types.APIUpdateServerSettingsRequest) error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return fmt.Errorf("error getting account model with error: %s", err.Error())
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return fmt.Errorf("error getting agent model with error: %s", err.Error())
	}

	if err := AccountModel.PopulateField(theAccount, "Agents"); err != nil {
		return fmt.Errorf("error populating account agents with error: %s", err.Error())
	}

	oid, err := primitive.ObjectIDFromHex(PostData.ID)
	if err != nil {
		return err
	}

	var theAgent *modelsv2.AgentSchema
	for idx := range theAccount.Agents {
		agent := &theAccount.Agents[idx]

		if agent.ID.Hex() == oid.Hex() {
			theAgent = agent
			break
		}
	}

	updateData := bson.M{}

	switch PostData.ConfigSetting {
	case "sfsettings":
		theAgent.ServerConfig.UpdateOnStart = (PostData.UpdateOnStart == "on")
		theAgent.ServerConfig.AutoRestart = (PostData.AutoRestart == "on")
		theAgent.ServerConfig.AutoPause = (PostData.AutoPause == "on")
		theAgent.ServerConfig.AutoSaveOnDisconnect = (PostData.AutoSaveOnDisconnect == "on")
		theAgent.ServerConfig.AutoSaveInterval = PostData.AutoSaveInterval
		theAgent.ServerConfig.DisableSeasonalEvents = PostData.SeasonalEvents != "on"
		theAgent.ServerConfig.MaxPlayers = PostData.MaxPlayers
		theAgent.ServerConfig.WorkerThreads = PostData.WorkerThreads
		if PostData.Branch == "on" {
			theAgent.ServerConfig.Branch = "experimental"
		} else {
			theAgent.ServerConfig.Branch = "public"
		}

		updateData["serverConfig"] = theAgent.ServerConfig

	case "backupsettings":
		theAgent.Config.BackupInterval = PostData.BackupInterval
		theAgent.Config.BackupKeepAmount = PostData.BackupKeep
		updateData["config"] = theAgent.Config
	case "modsettings":
		ModModel, err := repositories.GetMongoClient().GetModel("AgentModConfigSelectedMod")
		if err != nil {
			return err
		}

		for idx := range theAgent.ModConfig.SelectedMods {
			mod := &theAgent.ModConfig.SelectedMods[idx]
			if err := ModModel.PopulateField(mod, "Mod"); err != nil {
				return err
			}
		}

		var selectedMod *modelsv2.AgentModConfigSelectedModSchema
		for idx := range theAgent.ModConfig.SelectedMods {
			mod := &theAgent.ModConfig.SelectedMods[idx]
			if mod.Mod.ModReference == PostData.ModReference {
				selectedMod = mod
				break
			}
		}

		if selectedMod == nil {
			return errors.New("error cant find mod in selected mods list")
		}

		selectedMod.Config = PostData.ModConfig

		updateData["modConfig"] = theAgent.ModConfig

	default:
		return errors.New("error unknown config setting")
	}

	if len(updateData) == 0 {
		return nil
	}

	if err := AgentModel.UpdateData(theAgent, updateData); err != nil {
		return err
	}

	return nil
}

func GetAgentLog(theAgent *modelsv2.AgentSchema, logType string) (*modelsv2.AgentLogSchema, error) {
	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return nil, err
	}

	if err := AgentModel.PopulateField(theAgent, "Logs"); err != nil {
		return nil, err
	}

	for idx := range theAgent.Logs {
		log := &theAgent.Logs[idx]
		if log.Type == logType {
			return log, nil
		}
	}
	return &modelsv2.AgentLogSchema{}, nil
}

func CreateAgentTask(theAgent *modelsv2.AgentSchema, action string, data interface{}) error {
	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	newTask := modelsv2.NewAgentTask(action, data)
	theAgent.Tasks = append(theAgent.Tasks, newTask)

	updateData := bson.M{"tasks": theAgent.Tasks}
	if err := AgentModel.UpdateData(theAgent, updateData); err != nil {
		return err
	}
	return nil
}

func InstallMod(theAgent *modelsv2.AgentSchema, modReference string, version string) error {

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	ModModel, err := repositories.GetMongoClient().GetModel("Mod")
	if err != nil {
		return err
	}

	SelectedModModel, err := repositories.GetMongoClient().GetModel("AgentModConfigSelectedMod")
	if err != nil {
		return err
	}

	depResolver := resolver.NewDependencyResolver(utils.SSMProvider{})

	constraints := make(map[string]string, 0)

	constraints[modReference] = version

	requiredTargets := make([]resolver.TargetName, 0)
	requiredTargets = append(requiredTargets, resolver.TargetNameWindowsServer)
	requiredTargets = append(requiredTargets, resolver.TargetNameLinuxServer)

	resolved, err := depResolver.ResolveModDependencies(constraints, nil, math.MaxInt, requiredTargets)

	if err != nil {
		return err
	}

	for idx := range theAgent.ModConfig.SelectedMods {
		mod := &theAgent.ModConfig.SelectedMods[idx]
		if err := SelectedModModel.PopulateField(mod, "Mod"); err != nil {
			return err
		}
	}

	mods := resolved.Mods

	for k := range mods {
		mod := mods[k]

		exists := false
		for idx := range theAgent.ModConfig.SelectedMods {
			selectedMod := &theAgent.ModConfig.SelectedMods[idx]

			if selectedMod.Mod.ModReference == k {
				selectedMod.DesiredVersion = mod.Version
				exists = true
				break
			}
		}

		if !exists {

			var dbMod models.ModSchema
			if err := ModModel.FindOne(&dbMod, bson.M{"modReference": k}); err != nil {
				return err
			}

			fmt.Printf("Installing Mod %s\n", k)
			fmt.Printf("%+v\n", dbMod)

			newSelectedMod := modelsv2.AgentModConfigSelectedModSchema{
				ModId:            dbMod.ID,
				Mod:              dbMod,
				DesiredVersion:   mod.Version,
				InstalledVersion: "0.0.0",
				Config:           "{}",
			}

			theAgent.ModConfig.SelectedMods = append(theAgent.ModConfig.SelectedMods, newSelectedMod)
		}
	}

	dbUpdate := bson.M{
		"modConfig": theAgent.ModConfig,
		"updatedAt": time.Now(),
	}

	if err := AgentModel.UpdateData(theAgent, dbUpdate); err != nil {
		return err
	}

	return nil
}

func UpdateMod(theAgent *modelsv2.AgentSchema, modReference string) error {

	ModModel, err := repositories.GetMongoClient().GetModel("Mod")
	if err != nil {
		return err
	}

	var dbMod models.ModSchema

	if err := ModModel.FindOne(&dbMod, bson.M{"modReference": modReference}); err != nil {
		return fmt.Errorf("error finding mod with error: %s", err.Error())
	}

	if len(dbMod.Versions) == 0 {
		return errors.New("error updating mod with error: no mod versions")
	}

	latestVersion := dbMod.Versions[0].Version

	if err := InstallMod(theAgent, dbMod.ModReference, latestVersion); err != nil {
		return fmt.Errorf("error installing mod with error: %s", err.Error())
	}

	return nil
}

func UninstallMod(theAgent *modelsv2.AgentSchema, modReference string) error {

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	ModModel, err := repositories.GetMongoClient().GetModel("AgentModConfigSelectedMod")
	if err != nil {
		return err
	}

	for idx := range theAgent.ModConfig.SelectedMods {
		mod := &theAgent.ModConfig.SelectedMods[idx]
		if err := ModModel.PopulateField(mod, "Mod"); err != nil {
			return err
		}
	}

	newSelectedModsList := make([]modelsv2.AgentModConfigSelectedModSchema, 0)

	for idx := range theAgent.ModConfig.SelectedMods {
		selectedMod := theAgent.ModConfig.SelectedMods[idx]

		if selectedMod.Mod.ModReference != modReference {
			newSelectedModsList = append(newSelectedModsList, selectedMod)
		}
	}

	theAgent.ModConfig.SelectedMods = newSelectedModsList

	dbUpdate := bson.M{
		"modConfig": theAgent.ModConfig,
		"updatedAt": time.Now(),
	}

	if err := AgentModel.UpdateData(theAgent, dbUpdate); err != nil {
		return err
	}
	return nil
}

func AddAgentStat(theAgent *modelsv2.AgentSchema, runningState bool, cpu float64, memory float32) error {

	AgentStatModel, err := repositories.GetMongoClient().GetModel("AgentStat")
	if err != nil {
		return err
	}
	newStat := modelsv2.NewAgentStat(theAgent, runningState, cpu, memory)

	if err := AgentStatModel.Create(newStat); err != nil {
		return fmt.Errorf("error creating agent stat with error: %s", err.Error())
	}

	return nil
}

func PurgeAgentStats() error {

	now := time.Now()
	expiry := now.AddDate(0, 0, -3)

	AgentStatModel, err := repositories.GetMongoClient().GetModel("AgentStat")
	if err != nil {
		return err
	}

	filter := bson.M{"createdAt": bson.M{"$lt": expiry}}

	if err := AgentStatModel.Delete(filter); err != nil {
		return fmt.Errorf("error deleting agent stats with error: %s", err.Error())
	}

	return nil
}

func GetAgentStats(theAgent *modelsv2.AgentSchema) ([]*modelsv2.AgentStatSchema, error) {

	AgentStatModel, err := repositories.GetMongoClient().GetModel("AgentStat")
	if err != nil {
		return nil, err
	}

	stats := make([]*modelsv2.AgentStatSchema, 0)
	if err := AgentStatModel.FindAll(&stats, bson.M{"agentId": theAgent.ID}); err != nil {
		return nil, err
	}

	return stats, nil

}
