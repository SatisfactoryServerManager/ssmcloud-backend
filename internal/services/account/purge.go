package account

import (
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/audit"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// PurgeAccount fully removes an account and everything belonging to it: its
// agents, audits and integrations, its stored files, and the account record
// itself. It also unlinks the account from any users that reference it. This
// is the single source of truth for account deletion, used by both the admin
// delete endpoint and the inactive-account cleanup job.
func PurgeAccount(theAccount *models.AccountSchema) error {
	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return err
	}

	oid := theAccount.ID

	// Delete agents (also removes them from the account)
	for _, agentId := range theAccount.AgentIds {
		if id, ok := agentId.(bson.ObjectID); ok {
			_ = agent.DeleteAgent(theAccount, id)
		}
	}

	// Delete audits
	for _, auditId := range theAccount.AuditIds {
		if id, ok := auditId.(bson.ObjectID); ok {
			_ = audit.DeleteAccountAudit(theAccount, id)
		}
	}

	// Delete integrations
	for _, integrationId := range theAccount.IntegrationIds {
		if id, ok := integrationId.(bson.ObjectID); ok {
			_ = DeleteAccountIntegration(theAccount, id)
		}
	}

	// Unlink the account from every user that references it
	users := make([]models.UserSchema, 0)
	filter := bson.M{"linkedAccounts": bson.M{"$in": bson.A{oid}}}
	_ = UserModel.FindAll(&users, filter)
	for i := range users {
		u := &users[i]
		newLinked := make(bson.A, 0)
		for _, la := range u.LinkedAccountIds {
			if laId, ok := la.(bson.ObjectID); ok {
				if laId.Hex() != oid.Hex() {
					newLinked = append(newLinked, laId)
				}
			}
		}
		u.LinkedAccountIds = newLinked
		if u.ActiveAccountId.Hex() == oid.Hex() {
			if len(newLinked) > 0 {
				u.ActiveAccountId = newLinked[0].(bson.ObjectID)
			} else {
				u.ActiveAccountId = bson.NilObjectID
			}
		}
		_ = UserModel.UpdateData(u, bson.M{"linkedAccounts": u.LinkedAccountIds, "activeAccount": u.ActiveAccountId, "updatedAt": time.Now()})
	}

	// Remove stored files for this account (best-effort)
	_ = repositories.DeleteAccountFolder(oid.Hex())

	// Finally delete the account record itself
	return AccountModel.DeleteById(oid)
}
