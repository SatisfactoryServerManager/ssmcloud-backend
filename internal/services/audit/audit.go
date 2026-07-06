package audit

import (
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

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
		ID:        bson.NewObjectID(),
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

func DeleteAccountAudit(theAccount *models.AccountSchema, auditId bson.ObjectID) error {

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

	newAudits := make(bson.A, 0)
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
