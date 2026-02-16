package v2

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	goaway "github.com/TwiN/go-away"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func CreateAccount(theUser *models.UserSchema, accountName string) error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return err
	}

	if goaway.IsProfane(accountName) {
		return errors.New("error that account name is restricted")
	}

	existingAccount := &models.AccountSchema{}
	AccountModel.FindOne(existingAccount, bson.M{"accountName": accountName})

	if !existingAccount.ID.IsZero() {
		return errors.New("error that account name is already in use")
	}

	newAccount := models.NewAccount(accountName)
	if err := AccountModel.Create(newAccount); err != nil {
		return err
	}

	theUser.LinkedAccountIds = append(theUser.LinkedAccountIds, newAccount.ID)
	if theUser.ActiveAccountId.IsZero() {
		theUser.ActiveAccountId = newAccount.ID
	}

	updateData := bson.M{
		"linkedAccounts": theUser.LinkedAccountIds,
		"activeAccount":  theUser.ActiveAccountId,
	}

	if err := UserModel.UpdateData(theUser, updateData); err != nil {
		return err
	}

	if err := AddAccountAudit(newAccount,
		models.AuditType_UserAddedToAccount,
		fmt.Sprintf("User (%s) was added to the account", theUser.Username),
	); err != nil {
		return err
	}

	return nil
}

func JoinAccount(theUser *models.UserSchema, joinCode string) error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return err
	}

	existingAccount := &models.AccountSchema{}
	AccountModel.FindOne(existingAccount, bson.M{"joinCode": joinCode})

	if existingAccount.ID.IsZero() {
		return errors.New("account was not found")
	}

	for _, accountId := range theUser.LinkedAccountIds {
		if accountId.(primitive.ObjectID).Hex() == existingAccount.ID.Hex() {
			return errors.New("account is already linked")
		}
	}

	theUser.LinkedAccountIds = append(theUser.LinkedAccountIds, existingAccount.ID)
	theUser.ActiveAccountId = existingAccount.ID

	updateData := bson.M{
		"linkedAccounts": theUser.LinkedAccountIds,
		"activeAccount":  theUser.ActiveAccountId,
	}

	if err := UserModel.UpdateData(theUser, updateData); err != nil {
		return err
	}

	if err := AddAccountAudit(existingAccount,
		models.AuditType_UserAddedToAccount,
		fmt.Sprintf("User (%s) was added to the account", theUser.Username),
	); err != nil {
		return err
	}

	return nil
}

func SwitchAccount(theUser *models.UserSchema, accountId primitive.ObjectID) error {

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return err
	}

	found := false
	for _, id := range theUser.LinkedAccountIds {
		if id.(primitive.ObjectID).Hex() == accountId.Hex() {
			found = true
		}
	}

	if !found {
		return errors.New("account was not found")
	}

	theUser.ActiveAccountId = accountId

	updateData := bson.M{
		"activeAccount": theUser.ActiveAccountId,
	}

	if err := UserModel.UpdateData(theUser, updateData); err != nil {
		return err
	}

	return nil
}

func DeleteAccount(theUser *models.UserSchema, accountId primitive.ObjectID) error {

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return err
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	theAccount := &models.AccountSchema{}

	if err := AccountModel.FindOneById(theAccount, accountId); err != nil {
		return fmt.Errorf("error finding account with error: %s", err.Error())
	}

	for _, agentId := range theAccount.AgentIds {
		if err := DeleteAgent(theAccount, agentId.(primitive.ObjectID)); err != nil {
			return fmt.Errorf("error deleting account agent with error: %s", err.Error())
		}
	}

	for _, auditId := range theAccount.AuditIds {
		if err := DeleteAccountAudit(theAccount, auditId.(primitive.ObjectID)); err != nil {
			return fmt.Errorf("error deleting account agent with error: %s", err.Error())
		}
	}

	for _, integrationId := range theAccount.IntegrationIds {
		if err := DeleteAccountIntegration(theAccount, integrationId.(primitive.ObjectID)); err != nil {
			return fmt.Errorf("error deleting account agent with error: %s", err.Error())
		}
	}

	newUserAccounts := make(primitive.A, 0)
	for _, id := range theUser.LinkedAccountIds {
		aId := id.(primitive.ObjectID)
		if aId.Hex() != accountId.Hex() {
			newUserAccounts = append(newUserAccounts, aId)
		}
	}

	theUser.LinkedAccountIds = newUserAccounts

	if len(theUser.LinkedAccountIds) > 0 {
		if theUser.ActiveAccountId.Hex() == accountId.Hex() {
			theUser.ActiveAccountId = theUser.LinkedAccountIds[0].(primitive.ObjectID)
		}
	} else {
		theUser.ActiveAccountId = primitive.NilObjectID
	}

	if err := UserModel.UpdateData(theUser, bson.M{"linkedAccounts": theUser.LinkedAccountIds, "activeAccount": theUser.ActiveAccountId}); err != nil {
		return fmt.Errorf("error updating user removing account id with error: %s", err.Error())
	}

	return nil
}

func DeleteAccountAudit(theAccount *models.AccountSchema, auditId primitive.ObjectID) error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	AccountAuditModel, err := repositories.GetMongoClient().GetModel("AccountAudit")
	if err != nil {
		return err
	}

	if err := AccountModel.PopulateField(theAccount, "Audits"); err != nil {
		return fmt.Errorf("error populating account audits with error: %s", err.Error())
	}

	newAudits := make(primitive.A, 0)
	for _, audit := range theAccount.Audits {
		if audit.ID.Hex() != auditId.Hex() {
			newAudits = append(newAudits, audit.ID)
		}
	}

	theAccount.AuditIds = newAudits

	if err := AccountModel.UpdateData(theAccount, bson.M{"audit": theAccount.AuditIds}); err != nil {
		return fmt.Errorf("error removing audit from account with error: %s", err.Error())
	}

	if err := AccountAuditModel.DeleteById(auditId); err != nil {
		return fmt.Errorf("error deleting audit from db with error: %s", err.Error())
	}

	return nil
}

func GetUserActiveAccount(theUser *models.UserSchema) (*models.AccountSchema, error) {

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return nil, err
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return nil, err
	}

	if theUser.ActiveAccountId.IsZero() {
		return nil, nil
	}

	if err := UserModel.PopulateField(theUser, "ActiveAccount"); err != nil {
		return nil, fmt.Errorf("error populating active account with error: %s", err.Error())
	}

	if theUser.ActiveAccount.JoinCode == "" {
		theUser.ActiveAccount.JoinCode = utils.RandStringBytes(16)
		update := bson.M{"joinCode": theUser.ActiveAccount.JoinCode}
		if err := AccountModel.UpdateData(&theUser.ActiveAccount, update); err != nil {
			return nil, fmt.Errorf("error updating account join code with error: %s", err.Error())
		}
	}

	return &theUser.ActiveAccount, nil
}

func GetMyUserLinkedAccounts(theUser *models.UserSchema) ([]models.AccountSchema, error) {

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return nil, err
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return nil, err
	}

	if len(theUser.LinkedAccountIds) == 0 {
		emptyAccounts := make([]models.AccountSchema, 0)
		return emptyAccounts, nil
	}

	if err := UserModel.PopulateField(theUser, "LinkedAccounts"); err != nil {
		return nil, fmt.Errorf("error populating linked accounts with error: %s", err.Error())
	}

	for idx := range theUser.LinkedAccounts {
		account := &theUser.LinkedAccounts[idx]

		if account.JoinCode == "" {
			account.JoinCode = utils.RandStringBytes(16)
			update := bson.M{"joinCode": account.JoinCode}
			if err := AccountModel.UpdateData(account, update); err != nil {
				return nil, fmt.Errorf("error updating account join code with error: %s", err.Error())
			}
		}
	}

	return theUser.LinkedAccounts, nil
}

func GetUserAccountAudit(theUser *models.UserSchema) (*[]models.AccountAuditSchema, error) {

	theAccount, err := GetUserActiveAccount(theUser)
	if err != nil {
		return nil, err
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return nil, err
	}

	if err := AccountModel.PopulateField(theAccount, "Audits"); err != nil {
		return nil, err
	}

	return &theAccount.Audits, nil

}

func AddAccountAudit(theAccount *models.AccountSchema, auditType models.AuditType, message string) error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	AccountAuditModel, err := repositories.GetMongoClient().GetModel("AccountAudit")
	if err != nil {
		return err
	}

	newAudit := &models.AccountAuditSchema{
		ID:        primitive.NewObjectID(),
		Type:      auditType,
		Message:   message,
		CreatedAt: time.Now(),
	}

	if err := AccountAuditModel.Create(newAudit); err != nil {
		return fmt.Errorf("error creating account audit with error: %s", err.Error())
	}
	theAccount.AuditIds = append(theAccount.AuditIds, newAudit.ID)

	updateData := bson.M{
		"audit": theAccount.AuditIds,
	}

	if err := AccountModel.UpdateData(theAccount, updateData); err != nil {
		return fmt.Errorf("error updating account audits with error: %s", err.Error())
	}

	return nil
}

func GetUserAccountUsers(theAccount *models.AccountSchema) (*[]models.UserSchema, error) {
	accountId := theAccount.ID

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return nil, err
	}

	users := make([]models.UserSchema, 0)

	filter := bson.M{"linkedAccounts": bson.M{"$in": bson.A{accountId}}}
	if err := UserModel.FindAll(&users, filter); err != nil {
		return nil, err
	}

	return &users, nil
}

func GetMyAccountIntegrations(theAccount *models.AccountSchema) (*[]models.AccountIntegrationSchema, error) {
	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return nil, err
	}

	if err := AccountModel.PopulateField(theAccount, "Integrations"); err != nil {
		return nil, fmt.Errorf("error populating account integrations with error: %s", err.Error())
	}

	return &theAccount.Integrations, nil
}

func AddAccountIntegration(theAccount *models.AccountSchema, name string, integrationType models.IntegrationType, url string, eventTypes []models.IntegrationEventType) error {
	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	IntergrationsModel, err := repositories.GetMongoClient().GetModel("AccountIntegration")
	if err != nil {
		return err
	}

	if err := AccountModel.PopulateField(theAccount, "Integrations"); err != nil {
		return fmt.Errorf("error populating account integrations with error: %s", err.Error())
	}

	for _, integration := range theAccount.Integrations {
		if integration.Url == url {
			return fmt.Errorf("error integration with same url (%s) alreay exists", url)
		}
	}

	newIntegration := models.AccountIntegrationSchema{
		ID:         primitive.NewObjectID(),
		Name:       name,
		Type:       integrationType,
		Url:        url,
		EventTypes: eventTypes,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := IntergrationsModel.Create(newIntegration); err != nil {
		return fmt.Errorf("error creating integration with error: %s", err.Error())
	}

	theAccount.IntegrationIds = append(theAccount.IntegrationIds, newIntegration.ID)
	updateData := bson.M{
		"integrations": theAccount.IntegrationIds,
	}

	if err := AccountModel.UpdateData(theAccount, updateData); err != nil {
		return fmt.Errorf("error updating account integrations with error: %s", err.Error())
	}

	if err := AddAccountAudit(theAccount, models.AuditType_IntegrationAddedToAccount, "A new integration has been added to the account"); err != nil {
		return err
	}

	return nil
}

func UpdateAccountIntegration(integrationId primitive.ObjectID, name string, integrationType models.IntegrationType, url string, eventTypes []models.IntegrationEventType) error {
	IntergrationsModel, err := repositories.GetMongoClient().GetModel("AccountIntegration")
	if err != nil {
		return err
	}

	theIntegration := &models.AccountIntegrationSchema{}

	if err := IntergrationsModel.FindOneById(theIntegration, integrationId); err != nil {
		return fmt.Errorf("error finding integration with error: %s", err.Error())
	}

	updateData := bson.M{}
	if theIntegration.Name != name {
		updateData["name"] = name
	}

	if theIntegration.Type != integrationType {
		updateData["type"] = integrationType
	}

	if theIntegration.Url != url {
		updateData["url"] = url
	}

	if !reflect.DeepEqual(theIntegration.EventTypes, eventTypes) {
		updateData["eventTypes"] = eventTypes
	}

	if err := IntergrationsModel.UpdateData(theIntegration, updateData); err != nil {
		return err
	}

	return nil
}

func DeleteAccountIntegration(theAccount *models.AccountSchema, integrationId primitive.ObjectID) error {
	IntergrationsModel, err := repositories.GetMongoClient().GetModel("AccountIntegration")
	if err != nil {
		return err
	}

	IntergrationEventsModel, err := repositories.GetMongoClient().GetModel("IntegrationEvent")
	if err != nil {
		return err
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	if err := AccountModel.PopulateField(theAccount, "Integrations"); err != nil {
		return fmt.Errorf("error populating account integrations with error: %s", err.Error())
	}

	newIntegrations := make(primitive.A, 0)
	for _, integration := range theAccount.Integrations {
		if integration.ID.Hex() != integrationId.Hex() {
			newIntegrations = append(newIntegrations, integration.ID)
		}
	}

	theAccount.IntegrationIds = newIntegrations

	if err := AccountModel.UpdateData(theAccount, bson.M{"integrations": theAccount.IntegrationIds}); err != nil {
		return fmt.Errorf("error removing integrations from account with error: %s", err.Error())
	}

	if err := IntergrationsModel.DeleteById(integrationId); err != nil {
		return err
	}

	filter := bson.M{"integrationId": integrationId}
	if err := IntergrationEventsModel.Delete(filter); err != nil {
		return err
	}

	return nil
}
