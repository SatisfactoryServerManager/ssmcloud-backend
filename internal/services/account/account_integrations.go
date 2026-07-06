package account

import (
	"fmt"
	"reflect"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/audit"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

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
		ID:         bson.NewObjectID(),
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

	if err := audit.AddAccountAudit(theAccount, models.AuditType_IntegrationAddedToAccount, "A new integration has been added to the account"); err != nil {
		return err
	}

	return nil
}

func UpdateAccountIntegration(integrationId bson.ObjectID, name string, integrationType models.IntegrationType, url string, eventTypes []models.IntegrationEventType) error {
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

func DeleteAccountIntegration(theAccount *models.AccountSchema, integrationId bson.ObjectID) error {
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

	newIntegrations := make(bson.A, 0)
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
