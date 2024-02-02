package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/mircearoata/pubgrub-go/pubgrub/semver"
	"github.com/mrhid6/go-mongoose/mongoose"
	resolver "github.com/satisfactorymodding/ficsit-resolver"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func InitAgentService() {

	if err := CheckAllAgentsLastComms(); err != nil {
		fmt.Println(err)
	}

	if err := PurgeAgentTasks(); err != nil {
		fmt.Println(err)
	}

	if err := CheckAgentModsConfigs(); err != nil {
		fmt.Println(err)
	}

	uptimeticker := time.NewTicker(30 * time.Second)

	go func() {
		for {
			select {
			case <-uptimeticker.C:
				if err := CheckAllAgentsLastComms(); err != nil {
					fmt.Println(err)
				}
				if err := PurgeAgentTasks(); err != nil {
					fmt.Println(err)
				}
				if err := CheckAgentModsConfigs(); err != nil {
					fmt.Println(err)
				}
			case <-_quit:
				uptimeticker.Stop()
				log.Println("Stopped Process Orders Ticker")
				return
			}
		}
	}()
}

func ShutdownAgentService() error {
	return nil
}

func GetAllAgents(accountIdStr string) ([]models.Agents, error) {

	var theAccount models.Accounts
	emptyAgents := make([]models.Agents, 0)

	accountId, err := primitive.ObjectIDFromHex(accountIdStr)

	if err != nil {
		return emptyAgents, fmt.Errorf("error converting accountid to object id with error: %s", err.Error())
	}

	if err := mongoose.FindOne(bson.M{"_id": accountId}, &theAccount); err != nil {
		return emptyAgents, fmt.Errorf("error finding account from session with error: %s", err.Error())
	}

	if err := theAccount.PopulateAgents(); err != nil {
		return emptyAgents, fmt.Errorf("error populating account agents with error: %s", err.Error())
	}

	for idx := range theAccount.AgentObjects {
		agent := &theAccount.AgentObjects[idx]

		agent.PopulateModConfig()
	}

	return theAccount.AgentObjects, nil
}

func CreateAgent(accountIdStr string, agentName string, port int, memory int64) error {
	var theAccount models.Accounts

	accountId, err := primitive.ObjectIDFromHex(accountIdStr)

	if err != nil {
		return fmt.Errorf("error converting accountid to object id with error: %s", err.Error())
	}

	if err := mongoose.FindOne(bson.M{"_id": accountId}, &theAccount); err != nil {
		return fmt.Errorf("error finding account from session with error: %s", err.Error())
	}

	if err := theAccount.PopulateAgents(); err != nil {
		return fmt.Errorf("error populating account agents with error: %s", err.Error())
	}

	exists := false

	for _, agent := range theAccount.AgentObjects {
		if agent.AgentName == agentName {
			exists = true
			break
		}
	}

	if exists {
		return fmt.Errorf("error agent with same name %s already exists on your account", agentName)
	}

	newAgent := models.NewAgent(agentName, port, memory)

	if _, err := mongoose.InsertOne(&newAgent); err != nil {
		return fmt.Errorf("error inserting new agent with error: %s", err.Error())
	}

	theAccount.Agents = append(theAccount.Agents, newAgent.ID)

	dbUpdate := bson.D{{"$set", bson.D{
		{"agents", theAccount.Agents},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&theAccount, dbUpdate); err != nil {
		return fmt.Errorf("error updating account agents with error: %s", err.Error())
	}

	return nil
}

func GetAgentById(accountIdStr string, agentIdStr string) (models.Agents, error) {

	agents, err := GetAllAgents(accountIdStr)

	if err != nil {
		return models.Agents{}, err
	}

	agentId, err := primitive.ObjectIDFromHex(agentIdStr)

	if err != nil {
		return models.Agents{}, fmt.Errorf("error converting agentid to object id with error: %s", err.Error())
	}

	for _, agent := range agents {
		if agent.ID.Hex() == agentId.Hex() {
			return agent, nil
		}
	}

	return models.Agents{}, errors.New("error cant find agent on the account")
}

func GetAgentTasks(accountIdStr string, agentIdStr string) ([]models.AgentTask, error) {

	tasks := make([]models.AgentTask, 0)

	agent, err := GetAgentById(accountIdStr, agentIdStr)

	if err != nil {
		return tasks, err
	}

	return agent.Tasks, nil
}

func NewAgentTask(accountIdStr string, agentIdStr string, action string, data interface{}) error {

	newTask := models.NewAgentTask(action, data)

	agent, err := GetAgentById(accountIdStr, agentIdStr)

	if err != nil {
		return err
	}

	agent.Tasks = append(agent.Tasks, newTask)

	dbUpdate := bson.D{{"$set", bson.D{
		{"tasks", agent.Tasks},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&agent, dbUpdate); err != nil {
		return err
	}

	return nil
}

func DeleteAgent(accountIdStr string, agentIdStr string) error {

	theAgent, err := GetAgentById(accountIdStr, agentIdStr)

	if err != nil {
		return err
	}

	account, err := GetAccount(accountIdStr)
	if err != nil {
		return err
	}

	if err := account.PopulateAgents(); err != nil {
		return fmt.Errorf("error populating account agents with error: %s", err.Error())
	}

	newAgentList := make(primitive.A, 0)

	for _, agent := range account.AgentObjects {
		if agent.ID.Hex() != theAgent.ID.Hex() {
			newAgentList = append(newAgentList, agent.ID)
		}
	}

	dbUpdate := bson.D{{"$set", bson.D{
		{"agents", newAgentList},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&account, dbUpdate); err != nil {
		return err
	}

	if _, err := mongoose.DeleteOne(bson.M{"_id": theAgent.ID}, "agents"); err != nil {
		return err
	}

	return nil
}

func CheckAllAgentsLastComms() error {

	allAgents := make([]models.Agents, 0)

	if err := mongoose.FindAll(bson.M{}, &allAgents); err != nil {
		return err
	}

	for idx := range allAgents {
		agent := &allAgents[idx]

		d := time.Now().Add(-1 * time.Hour)

		if agent.Status.LastCommDate.Before(d) {
			if agent.Status.Online {
				agent.Status.Online = false
				agent.Status.Running = false
				agent.Status.CPU = 0
				agent.Status.RAM = 0
				dbUpdate := bson.D{{"$set", bson.D{
					{"status", agent.Status},
					{"updatedAt", time.Now()},
				}}}

				if err := mongoose.UpdateDataByID(agent, dbUpdate); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func PurgeAgentTasks() error {

	allAgents := make([]models.Agents, 0)

	if err := mongoose.FindAll(bson.M{}, &allAgents); err != nil {
		return err
	}

	for idx := range allAgents {
		agent := &allAgents[idx]

		if err := agent.PurgeTasks(); err != nil {
			return err
		}

	}

	return nil
}

func CheckAgentModsConfigs() error {

	agents := make([]models.Agents, 0)

	if err := mongoose.FindAll(bson.M{}, &agents); err != nil {
		return fmt.Errorf("error finding agents with error: %s", err.Error())
	}

	for idx := range agents {
		agent := &agents[idx]

		agent.PopulateModConfig()

		for modidx := range agent.ModConfig.SelectedMods {
			selectedMod := &agent.ModConfig.SelectedMods[modidx]

			if len(selectedMod.ModObject.Versions) == 0 {
				continue
			}

			latestVersion, _ := semver.NewVersion(selectedMod.ModObject.Versions[0].Version)

			//installedVersion, _ := semver.NewVersion(selectedMod.InstalledVersion)
			desiredVersion, _ := semver.NewVersion(selectedMod.DesiredVersion)

			if latestVersion.Compare(desiredVersion) == 0 {
				selectedMod.NeedsUpdate = false
			} else if latestVersion.Compare(desiredVersion) > 0 {
				selectedMod.NeedsUpdate = true
			}
		}

		dbUpdate := bson.D{{"$set", bson.D{
			{"modConfig", agent.ModConfig},
			{"updatedAt", time.Now()},
		}}}

		if err := mongoose.UpdateDataByID(agent, dbUpdate); err != nil {
			return err
		}
	}

	return nil
}

func InstallMod(accountIdStr string, agentIdStr, modReference string, version string) error {

	agent, err := GetAgentById(accountIdStr, agentIdStr)
	if err != nil {
		return err
	}

	depResolver := resolver.NewDependencyResolver(SSMProvider{}, "https://api.ficsit.app")

	constraints := make(map[string]string, 0)

	constraints[modReference] = version

	requiredTargets := make([]resolver.TargetName, 0)
	requiredTargets = append(requiredTargets, resolver.TargetNameWindowsServer)
	requiredTargets = append(requiredTargets, resolver.TargetNameLinuxServer)

	resolved, err := depResolver.ResolveModDependencies(context.Background(), constraints, nil, math.MaxInt, requiredTargets)

	if err != nil {
		return err
	}

	mods := resolved.Mods

	for k := range mods {
		mod := mods[k]

		if k == "SML" {

			smlConstraint, err := semver.NewConstraint(">" + agent.ModConfig.InstalledSMLVersion)
			if err != nil {
				return err
			}

			smlVersion, err := semver.NewVersion(mod.Version)
			if err != nil {
				return err
			}

			if smlConstraint.Contains(smlVersion) {
				agent.ModConfig.LatestSMLVersion = smlVersion.RawString()
			}
			continue
		}

		exists := false
		for idx := range agent.ModConfig.SelectedMods {
			selectedMod := &agent.ModConfig.SelectedMods[idx]

			if selectedMod.ModObject.ModReference == k {
				selectedMod.DesiredVersion = mod.Version
				exists = true
				break
			}
		}

		if !exists {

			fmt.Printf("Installing Mod %s\n", k)

			var dbMod models.Mods
			if err := mongoose.FindOne(bson.M{"modReference": k}, &dbMod); err != nil {
				return err
			}

			newSelectedMod := models.AgentModConfigSelectedMod{
				Mod:              dbMod.ID,
				ModObject:        dbMod,
				DesiredVersion:   mod.Version,
				InstalledVersion: "0.0.0",
				Config:           "{}",
			}

			agent.ModConfig.SelectedMods = append(agent.ModConfig.SelectedMods, newSelectedMod)
		}
	}

	dbUpdate := bson.D{{"$set", bson.D{
		{"modConfig", agent.ModConfig},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&agent, dbUpdate); err != nil {
		return err
	}

	return nil
}

func UpdateMod(accountIdStr string, agentIdStr, modReference string) error {

	var dbMod models.Mods

	if err := mongoose.FindOne(bson.M{"modReference": modReference}, &dbMod); err != nil {
		return fmt.Errorf("error finding mod with error: %s", err.Error())
	}

	if len(dbMod.Versions) == 0 {
		return errors.New("error updating mod with error: no mod versions")
	}

	latestVersion := dbMod.Versions[0].Version

	if err := InstallMod(accountIdStr, agentIdStr, dbMod.ModReference, latestVersion); err != nil {
		return fmt.Errorf("error installing mod with error: %s", err.Error())
	}

	return nil
}

func UninstallMod(accountIdStr string, agentIdStr, modReference string) error {

	agent, err := GetAgentById(accountIdStr, agentIdStr)
	if err != nil {
		return err
	}

	newSelectedModsList := make([]models.AgentModConfigSelectedMod, 0)

	for idx := range agent.ModConfig.SelectedMods {
		selectedMod := agent.ModConfig.SelectedMods[idx]

		if selectedMod.ModObject.ModReference != modReference {
			newSelectedModsList = append(newSelectedModsList, selectedMod)
		}
	}

	agent.ModConfig.SelectedMods = newSelectedModsList

	dbUpdate := bson.D{{"$set", bson.D{
		{"modConfig", agent.ModConfig},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&agent, dbUpdate); err != nil {
		return err
	}
	return nil
}
