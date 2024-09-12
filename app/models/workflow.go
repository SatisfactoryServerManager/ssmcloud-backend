package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/logger"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type API_AccountCreateAgent_PostData struct {
	AccountId  primitive.ObjectID `json:"accountid" bson:"accountId"`
	AgentName  string             `json:"servername" bson:"serverName"`
	Port       int                `json:"serverPort" bson:"serverPort"`
	Memory     int64              `json:"serverMemory" bson:"serverMemory"`
	AdminPass  string             `json:"serverAdminPass" bson:"serverAdminPass"`
	ClientPass string             `json:"serverClientPass" bson:"serverClientPass"`
	APIKey     string             `json:"serverAPIKey" bson:"serverApiKey"`
}

type ClaimServer_PostData struct {
	AdminPass  string `json:"adminPass"`
	ClientPass string `json:"clientPass"`
}

type Workflows struct {
	ID      primitive.ObjectID `json:"_id" bson:"_id"`
	Type    string             `json:"type" bson:"type"`
	Actions []WorkflowAction   `json:"actions" bson:"actions"`
	Status  string             `json:"status" bson:"status"`
	Data    interface{}        `json:"data" bson:"data"`
}

type WorkflowAction struct {
	Type         string `json:"type" bson:"type"`
	Status       string `json:"status" bson:"status"`
	ErrorMessage string `json:"error" bson:"error"`
	RetryCount   int    `json:"retryCount" bson:"retryCount"`
}

func (obj *Workflows) ValidateStatus() {
	completed := true
	failed := false

	for actionIdx := range obj.Actions {
		action := &obj.Actions[actionIdx]
		if action.Status == "" {
			completed = false
			break
		} else if action.Status == "failed" {
			failed = true
			break
		}
	}

	if completed {
		obj.Status = "completed"
		return
	}

	if failed {
		obj.Status = "failed"
		return
	}
}

func (obj *Workflows) ProcessCurrentAction() {

	currentActionIndex := 0

	for idx := range obj.Actions {
		action := &obj.Actions[idx]

		if action.Status == "" {
			currentActionIndex = idx
			break
		}
	}
	workflowData := API_AccountCreateAgent_PostData{}
	bodyBytes, _ := json.Marshal(obj.Data)
	json.Unmarshal(bodyBytes, &workflowData)

	//

	action := &obj.Actions[currentActionIndex]
	var theAccount Accounts

	if err := mongoose.FindOne(bson.M{"_id": workflowData.AccountId}, &theAccount); err != nil {
		logger.GetErrorLogger().Printf("error finding account from workflow with error %s\n", err.Error())
		return
	}

	if err := theAccount.PopulateAgents(); err != nil {
		logger.GetErrorLogger().Printf("error failed to populate agents from workflow with error %s\n", err.Error())
		return
	}

	if action.Type == "create-agent" {
		if err := action.CreateAgent(workflowData, &theAccount); err != nil {
			action.Status = "failed"
			action.ErrorMessage = err.Error()
		} else {
			action.Status = "completed"
		}
	} else if action.Type == "wait-for-online" {
		online, err := action.WaitForOnline(workflowData)

		if online {
			action.Status = "completed"
		} else {
			if err != nil {
				action.Status = "failed"
				action.ErrorMessage = err.Error()
			}
		}
	} else if action.Type == "install-server" {
		if err := action.InstallSFServer(workflowData); err != nil {
			action.Status = "failed"
			action.ErrorMessage = err.Error()
		} else {
			action.Status = "completed"
		}

	} else if action.Type == "wait-for-installed" {
		installed, err := action.WaitForInstalled(workflowData)

		if installed {
			action.Status = "completed"
		} else {
			if err != nil {
				action.Status = "failed"
				action.ErrorMessage = err.Error()
			}
		}
	} else if action.Type == "start-server" {
		if err := action.StartSFServer(workflowData); err != nil {
			action.Status = "failed"
			action.ErrorMessage = err.Error()
		} else {
			action.Status = "completed"
		}

	} else if action.Type == "wait-for-running" {
		running, err := action.WaitForRunning(workflowData)

		if running {
			action.Status = "completed"
		} else {
			if err != nil {
				action.Status = "failed"
				action.ErrorMessage = err.Error()
			}
		}
	} else if action.Type == "claim-server" {
		if err := action.ClaimServer(workflowData); err != nil {
			action.Status = "failed"
			action.ErrorMessage = err.Error()
		} else {
			action.Status = "completed"
		}
	} else {
		fmt.Printf("unknown workflow action %s\n", action.Type)
	}

	obj.ValidateStatus()

	dbUpdate := bson.D{{"$set", bson.D{
		{"status", obj.Status},
		{"actions", obj.Actions},
	}}}

	if err := mongoose.UpdateDataByID(*obj, dbUpdate); err != nil {
		fmt.Println(err.Error())
	}
}

func (obj *WorkflowAction) CreateAgent(workflowData API_AccountCreateAgent_PostData, theAccount *Accounts) error {
	newAgent := NewAgent(workflowData.AgentName, workflowData.Port, workflowData.Memory, workflowData.APIKey)

	if _, err := mongoose.InsertOne(&newAgent); err != nil {
		return fmt.Errorf("error inserting new agent with error: %s", err.Error())
	}

	theAccount.Agents = append(theAccount.Agents, newAgent.ID)

	dbUpdate := bson.D{{"$set", bson.D{
		{"agents", theAccount.Agents},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(*theAccount, dbUpdate); err != nil {
		return fmt.Errorf("error updating account agents with error: %s", err.Error())
	}

	theAccount.AddAudit("CREATE_AGENT", fmt.Sprintf("New agent created (%s)", workflowData.AgentName))
	return nil
}

func (obj *WorkflowAction) WaitForOnline(workflowData API_AccountCreateAgent_PostData) (bool, error) {
	var theAgent Agents

	if err := mongoose.FindOne(bson.M{"apiKey": workflowData.APIKey}, &theAgent); err != nil {
		return false, err
	}

	fmt.Printf("waiting for agent: %s to be online \n", theAgent.AgentName)

	if !theAgent.Status.Online {
		obj.RetryCount += 1
		if obj.RetryCount > 120 {
			return false, fmt.Errorf("timeout waiting for agent to start")
		}

		return false, nil
	}

	return true, nil
}

func (obj *WorkflowAction) InstallSFServer(workflowData API_AccountCreateAgent_PostData) error {

	var theAgent Agents

	if err := mongoose.FindOne(bson.M{"apiKey": workflowData.APIKey}, &theAgent); err != nil {
		return err
	}

	newTask := NewAgentTask("installsfserver", nil)

	theAgent.Tasks = append(theAgent.Tasks, newTask)

	dbUpdate := bson.D{{"$set", bson.D{
		{"tasks", theAgent.Tasks},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&theAgent, dbUpdate); err != nil {
		return err
	}

	return nil
}

func (obj *WorkflowAction) WaitForInstalled(workflowData API_AccountCreateAgent_PostData) (bool, error) {
	var theAgent Agents

	if err := mongoose.FindOne(bson.M{"apiKey": workflowData.APIKey}, &theAgent); err != nil {
		return false, err
	}

	fmt.Printf("waiting for agent: %s to install sf server \n", theAgent.AgentName)

	if !theAgent.Status.Installed {
		obj.RetryCount += 1
		if obj.RetryCount > 120 {
			return false, fmt.Errorf("timeout waiting for agent to install sf server")
		}

		return false, nil
	}

	return true, nil
}

func (obj *WorkflowAction) StartSFServer(workflowData API_AccountCreateAgent_PostData) error {

	var theAgent Agents

	if err := mongoose.FindOne(bson.M{"apiKey": workflowData.APIKey}, &theAgent); err != nil {
		return err
	}

	newTask := NewAgentTask("startsfserver", nil)

	theAgent.Tasks = append(theAgent.Tasks, newTask)

	dbUpdate := bson.D{{"$set", bson.D{
		{"tasks", theAgent.Tasks},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&theAgent, dbUpdate); err != nil {
		return err
	}

	return nil
}

func (obj *WorkflowAction) WaitForRunning(workflowData API_AccountCreateAgent_PostData) (bool, error) {
	var theAgent Agents

	if err := mongoose.FindOne(bson.M{"apiKey": workflowData.APIKey}, &theAgent); err != nil {
		return false, err
	}

	fmt.Printf("waiting for agent: %s to start sf server \n", theAgent.AgentName)

	if !theAgent.Status.Running {
		obj.RetryCount += 1
		if obj.RetryCount > 120 {
			return false, fmt.Errorf("timeout waiting for agent to start sf server")
		}

		return false, nil
	}

	return true, nil
}

func (obj *WorkflowAction) ClaimServer(workflowData API_AccountCreateAgent_PostData) error {
	var theAgent Agents

	if err := mongoose.FindOne(bson.M{"apiKey": workflowData.APIKey}, &theAgent); err != nil {
		return err
	}

	data := ClaimServer_PostData{
		AdminPass:  workflowData.AdminPass,
		ClientPass: workflowData.ClientPass,
	}

	newTask := NewAgentTask("claimserver", data)

	theAgent.Tasks = append(theAgent.Tasks, newTask)

	dbUpdate := bson.D{{"$set", bson.D{
		{"tasks", theAgent.Tasks},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&theAgent, dbUpdate); err != nil {
		return err
	}

	return nil
}
