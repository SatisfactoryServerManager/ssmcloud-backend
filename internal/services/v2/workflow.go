package v2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/mrhid6/go-mongoose-lock/joblock"
	"go.mongodb.org/mongo-driver/bson"
)

var workflowActionRegistry = map[string]v2.IWorkflowAction{}

type CreateAgentAction struct{}
type WaitForOnlineAction struct{}
type InstallServerAction struct{}
type WaitForInstallAction struct{}
type StartServerAction struct{}
type WaitForRunningAction struct{}
type ClaimServerAction struct{}

var (
	processWorkflowsJob *joblock.JobLockTask
)

func InitWorkflowService() {

	RegisterWorkflowAction(v2.WorkflowActionType_CreateAgent, CreateAgentAction{})
	RegisterWorkflowAction(v2.WorkflowActionType_WaitForOnline, WaitForOnlineAction{})
	RegisterWorkflowAction(v2.WorkflowActionType_InstallServer, InstallServerAction{})
	RegisterWorkflowAction(v2.WorkflowActionType_WaitForInstalled, WaitForInstallAction{})
	RegisterWorkflowAction(v2.WorkflowActionType_StartServer, StartServerAction{})
	RegisterWorkflowAction(v2.WorkflowActionType_WaitForRunning, WaitForRunningAction{})
	RegisterWorkflowAction(v2.WorkflowActionType_ClaimServer, ClaimServerAction{})

	processWorkflowsJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"processWorkflowsJob", func() {
			if err := ProcessWorkflows(); err != nil {
				logger.GetErrorLogger().Printf("error running account workflow job with error: %s", err.Error())
			}
		},
		5*time.Second,
		10*time.Second,
		false,
	)

	ctx := context.Background()

	if err := processWorkflowsJob.Run(ctx); err != nil {
		fmt.Printf("%v\n", err.Error())
	}

	logger.GetDebugLogger().Println("Initalized Workflow Service")
}

func ShutdownWorkflowService() error {
	processWorkflowsJob.UnLock(context.TODO())

	logger.GetDebugLogger().Println("Shutdown Workflow Service")
	return nil
}

func RegisterWorkflowAction(name string, handler v2.IWorkflowAction) {
	if workflowActionRegistry == nil {
		workflowActionRegistry = map[string]v2.IWorkflowAction{}
	}
	workflowActionRegistry[name] = handler
}

func ProcessWorkflows() error {

	WorkflowModel, err := repositories.GetMongoClient().GetModel("Workflow")
	if err != nil {
		return err
	}

	workflows := make([]v2.WorkflowSchema, 0)

	if err := WorkflowModel.FindAll(&workflows, bson.M{"status": ""}); err != nil {
		return err
	}

	if len(workflows) == 0 {
		return nil
	}

	fmt.Println("Processing Workflows")

	for idx := range workflows {
		workflow := &workflows[idx]

		ValidateStatus(workflow)
		if workflow.Status != "" {
			continue
		}

		if err := ProcessWorkflow(workflow); err != nil {
			return err
		}
	}

	return nil
}

func ValidateStatus(obj *v2.WorkflowSchema) {
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

func ProcessWorkflow(workflow *v2.WorkflowSchema) error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	WorkflowModel, err := repositories.GetMongoClient().GetModel("Workflow")
	if err != nil {
		return err
	}

	workflowData := v2.BaseWorkflowData{}
	bodyBytes, _ := json.Marshal(workflow.Data)
	json.Unmarshal(bodyBytes, &workflowData)

	theAccount := &v2.AccountSchema{}

	if err := AccountModel.FindOne(theAccount, bson.M{"_id": workflowData.AccountId}); err != nil {
		return fmt.Errorf("error finding account from workflow with error %s", err.Error())
	}

	if err := AccountModel.PopulateField(theAccount, "Agents"); err != nil {
		return fmt.Errorf("error failed to populate agents from workflow with error %s", err.Error())
	}

	switch workflow.Type {
	case v2.WorkflowType_CreateAgent:
		processWorkflow_CreateAgent(workflow, theAccount)
	default:
		return errors.New("unknown workflow type")
	}

	ValidateStatus(workflow)

	dbUpdate := bson.M{
		"status":  workflow.Status,
		"actions": workflow.Actions,
	}

	if err := WorkflowModel.UpdateData(workflow, dbUpdate); err != nil {
		return fmt.Errorf("error updating workflow with error: %s", err.Error())
	}

	return nil
}

func processWorkflow_CreateAgent(workflow *v2.WorkflowSchema, theAccount *v2.AccountSchema) {

	workflowData := v2.CreateAgentWorkflowData{}
	bodyBytes, _ := json.Marshal(workflow.Data)
	json.Unmarshal(bodyBytes, &workflowData)

	currentActionIndex := 0

	for idx := range workflow.Actions {
		action := &workflow.Actions[idx]

		if action.Status == "" {
			currentActionIndex = idx
			break
		}
	}

	action := &workflow.Actions[currentActionIndex]

	executeWorkflowAction(action, &workflowData, theAccount)
}

func executeWorkflowAction(action *v2.WorkflowAction, d interface{}, theAccount *v2.AccountSchema) {
	handler, ok := workflowActionRegistry[action.Type]
	if !ok {
		action.Status = "failed"
		action.ErrorMessage = "unknown action type: " + action.Type

	}

	err := handler.Execute(action, d, theAccount)
	if err != nil {
		action.Status = "failed"
		action.ErrorMessage = err.Error()
	}
}

func (a CreateAgentAction) Execute(action *v2.WorkflowAction, d interface{}, theAccount *v2.AccountSchema) error {

	workflowData := d.(*v2.CreateAgentWorkflowData)

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	newAgent := v2.NewAgent(workflowData.AgentName, workflowData.Port, workflowData.Memory, workflowData.APIKey)

	if err := AgentModel.Create(newAgent); err != nil {
		return fmt.Errorf("error inserting new agent with error: %s", err.Error())
	}

	theAccount.AgentIds = append(theAccount.AgentIds, newAgent.ID)

	dbUpdate := bson.M{
		"agents":    theAccount.AgentIds,
		"updatedAt": time.Now(),
	}

	if err := AccountModel.UpdateData(theAccount, dbUpdate); err != nil {
		return fmt.Errorf("error updating account AgentSchema with error: %s", err.Error())
	}

	if err := AddAccountAudit(theAccount,
		v2.AuditType_AgentAddedToAccount,
		fmt.Sprintf("Agent (%s) was added to the account", newAgent.AgentName),
	); err != nil {
		return err
	}

	action.Status = "completed"
	return nil
}

func (a WaitForOnlineAction) Execute(action *v2.WorkflowAction, d interface{}, theAccount *v2.AccountSchema) error {

	workflowData := d.(*v2.CreateAgentWorkflowData)

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAgent := &v2.AgentSchema{}

	if err := AgentModel.FindOne(theAgent, bson.M{"apiKey": workflowData.APIKey}); err != nil {
		return err
	}

	fmt.Printf("waiting for agent: %s to be online \n", theAgent.AgentName)

	if !theAgent.Status.Online {
		action.RetryCount += 1
		if action.RetryCount > 300 {
			return fmt.Errorf("timeout waiting for agent to start")
		}
		action.Status = ""
		return nil
	}

	action.Status = "completed"

	return nil
}

func (a InstallServerAction) Execute(action *v2.WorkflowAction, d interface{}, theAccount *v2.AccountSchema) error {

	workflowData := d.(*v2.CreateAgentWorkflowData)

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAgent := &v2.AgentSchema{}

	if err := AgentModel.FindOne(theAgent, bson.M{"apiKey": workflowData.APIKey}); err != nil {
		return err
	}

	newTask := v2.NewAgentTask("installsfserver", nil)

	theAgent.Tasks = append(theAgent.Tasks, newTask)

	dbUpdate := bson.M{
		"tasks":     theAgent.Tasks,
		"updatedAt": time.Now(),
	}

	if err := AgentModel.UpdateData(theAgent, dbUpdate); err != nil {
		return err
	}

	action.Status = "completed"

	return nil
}

func (a WaitForInstallAction) Execute(action *v2.WorkflowAction, d interface{}, theAccount *v2.AccountSchema) error {
	workflowData := d.(*v2.CreateAgentWorkflowData)

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAgent := &v2.AgentSchema{}

	if err := AgentModel.FindOne(theAgent, bson.M{"apiKey": workflowData.APIKey}); err != nil {
		return err
	}

	fmt.Printf("waiting for agent: %s to install sf server \n", theAgent.AgentName)

	if !theAgent.Status.Installed {
		action.RetryCount += 1
		if action.RetryCount > 300 {
			return fmt.Errorf("timeout waiting for agent to install sf server")
		}
		action.Status = ""
		return nil
	}
	action.Status = "completed"

	return nil
}

func (a StartServerAction) Execute(action *v2.WorkflowAction, d interface{}, theAccount *v2.AccountSchema) error {

	workflowData := d.(*v2.CreateAgentWorkflowData)

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAgent := &v2.AgentSchema{}

	if err := AgentModel.FindOne(theAgent, bson.M{"apiKey": workflowData.APIKey}); err != nil {
		return err
	}

	newTask := v2.NewAgentTask("startsfserver", nil)

	theAgent.Tasks = append(theAgent.Tasks, newTask)

	dbUpdate := bson.M{
		"tasks":     theAgent.Tasks,
		"updatedAt": time.Now(),
	}

	if err := AgentModel.UpdateData(theAgent, dbUpdate); err != nil {
		return err
	}

	action.Status = "completed"

	return nil
}

func (a WaitForRunningAction) Execute(action *v2.WorkflowAction, d interface{}, theAccount *v2.AccountSchema) error {
	workflowData := d.(*v2.CreateAgentWorkflowData)

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAgent := &v2.AgentSchema{}

	if err := AgentModel.FindOne(theAgent, bson.M{"apiKey": workflowData.APIKey}); err != nil {
		return err
	}

	fmt.Printf("waiting for agent: %s to start sf server \n", theAgent.AgentName)

	if !theAgent.Status.Running {
		action.RetryCount += 1
		if action.RetryCount > 300 {
			return fmt.Errorf("timeout waiting for agent to start sf server")
		}
		action.Status = ""
		return nil
	}
	action.Status = "completed"

	return nil
}

func (a ClaimServerAction) Execute(action *v2.WorkflowAction, d interface{}, theAccount *v2.AccountSchema) error {
	workflowData := d.(*v2.CreateAgentWorkflowData)

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAgent := &v2.AgentSchema{}

	if err := AgentModel.FindOne(theAgent, bson.M{"apiKey": workflowData.APIKey}); err != nil {
		return err
	}

	data := v2.ClaimServer_PostData{
		AdminPass:  workflowData.AdminPass,
		ClientPass: workflowData.ClientPass,
	}

	newTask := v2.NewAgentTask("claimserver", data)

	theAgent.Tasks = append(theAgent.Tasks, newTask)

	dbUpdate := bson.M{
		"tasks":     theAgent.Tasks,
		"updatedAt": time.Now(),
	}

	if err := AgentModel.UpdateData(theAgent, dbUpdate); err != nil {
		return err
	}

	action.Status = "completed"

	return nil
}
