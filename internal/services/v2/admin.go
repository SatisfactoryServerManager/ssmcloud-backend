package v2

import (
	"errors"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	defaultAdminPageSize = 50
	maxAdminPageSize     = 500
)

func normalizePaging(page, pageSize int32) (int, int) {
	p := int(page)
	ps := int(pageSize)
	if p <= 0 {
		p = 1
	}
	if ps <= 0 {
		ps = defaultAdminPageSize
	}
	if ps > maxAdminPageSize {
		ps = maxAdminPageSize
	}
	return p, ps
}

func paginateSlice[T any](in []T, page, pageSize int) ([]T, int) {
	total := len(in)
	start := (page - 1) * pageSize
	if start >= total {
		return []T{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return in[start:end], total
}

// ---- Users ----

func AdminGetUser(userId, externalId, email string) (*models.UserSchema, error) {
	var oid bson.ObjectID
	var err error
	if userId != "" {
		oid, err = bson.ObjectIDFromHex(userId)
		if err != nil {
			return nil, fmt.Errorf("invalid user_id")
		}
	}

	if userId == "" && externalId == "" && email == "" {
		return nil, errors.New("missing identifier")
	}

	return GetUser(oid, externalId, email, "")
}

func AdminListUsers(page, pageSize int32, search string) ([]models.UserSchema, int, error) {
	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return nil, 0, err
	}

	users := make([]models.UserSchema, 0)
	filter := bson.M{}
	if search != "" {
		filter = bson.M{"$or": bson.A{
			bson.M{"email": bson.M{"$regex": search, "$options": "i"}},
			bson.M{"username": bson.M{"$regex": search, "$options": "i"}},
			bson.M{"eid": bson.M{"$regex": search, "$options": "i"}},
		}}
	}

	if err := UserModel.FindAll(&users, filter); err != nil {
		return nil, 0, err
	}

	p, ps := normalizePaging(page, pageSize)
	paged, total := paginateSlice(users, p, ps)
	return paged, total, nil
}

func AdminUpdateUser(userId, externalId, email, username string) (*models.UserSchema, error) {
	if userId == "" {
		return nil, errors.New("missing user_id")
	}

	oid, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id")
	}

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return nil, err
	}

	theUser := &models.UserSchema{}
	if err := UserModel.FindOneById(theUser, oid); err != nil {
		return nil, err
	}

	update := bson.M{}
	if externalId != "" && externalId != theUser.ExternalID {
		theUser.ExternalID = externalId
		update["eid"] = externalId
	}
	if email != "" && email != theUser.Email {
		theUser.Email = email
		update["email"] = email
	}
	if username != "" && username != theUser.Username {
		theUser.Username = username
		update["username"] = username
	}
	if len(update) == 0 {
		return theUser, nil
	}

	theUser.UpdatedAt = time.Now()
	update["updatedAt"] = theUser.UpdatedAt

	if err := UserModel.UpdateData(theUser, update); err != nil {
		return nil, err
	}

	return theUser, nil
}

func AdminDeleteUser(userId string) error {
	if userId == "" {
		return errors.New("missing user_id")
	}
	oid, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return fmt.Errorf("invalid user_id")
	}

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return err
	}

	return UserModel.DeleteById(oid)
}

// ---- Accounts ----

func AdminGetAccount(accountId string) (*models.AccountSchema, error) {
	if accountId == "" {
		return nil, errors.New("missing account_id")
	}

	oid, err := bson.ObjectIDFromHex(accountId)
	if err != nil {
		return nil, fmt.Errorf("invalid account_id")
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return nil, err
	}

	theAccount := &models.AccountSchema{}
	if err := AccountModel.FindOneById(theAccount, oid); err != nil {
		return nil, err
	}

	return theAccount, nil
}

func AdminListAccounts(page, pageSize int32, search string) ([]models.AccountSchema, int, error) {
	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return nil, 0, err
	}

	accounts := make([]models.AccountSchema, 0)
	filter := bson.M{}
	if search != "" {
		filter = bson.M{"$or": bson.A{
			bson.M{"accountName": bson.M{"$regex": search, "$options": "i"}},
			bson.M{"joinCode": bson.M{"$regex": search, "$options": "i"}},
		}}
	}

	if err := AccountModel.FindAll(&accounts, filter); err != nil {
		return nil, 0, err
	}

	p, ps := normalizePaging(page, pageSize)
	paged, total := paginateSlice(accounts, p, ps)
	return paged, total, nil
}

func AdminUpdateAccount(accountId, accountName string) (*models.AccountSchema, error) {
	if accountId == "" {
		return nil, errors.New("missing account_id")
	}

	oid, err := bson.ObjectIDFromHex(accountId)
	if err != nil {
		return nil, fmt.Errorf("invalid account_id")
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return nil, err
	}

	theAccount := &models.AccountSchema{}
	if err := AccountModel.FindOneById(theAccount, oid); err != nil {
		return nil, err
	}

	update := bson.M{}
	if accountName != "" && accountName != theAccount.AccountName {
		theAccount.AccountName = accountName
		update["accountName"] = accountName
	}
	if len(update) == 0 {
		return theAccount, nil
	}

	theAccount.UpdatedAt = time.Now()
	update["updatedAt"] = theAccount.UpdatedAt

	if err := AccountModel.UpdateData(theAccount, update); err != nil {
		return nil, err
	}

	return theAccount, nil
}

func AdminDeleteAccount(accountId string) error {
	if accountId == "" {
		return errors.New("missing account_id")
	}

	oid, err := bson.ObjectIDFromHex(accountId)
	if err != nil {
		return fmt.Errorf("invalid account_id")
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return err
	}

	theAccount := &models.AccountSchema{}
	if err := AccountModel.FindOneById(theAccount, oid); err != nil {
		return err
	}

	// Delete agents (also removes from account)
	for _, agentId := range theAccount.AgentIds {
		if id, ok := agentId.(bson.ObjectID); ok {
			_ = DeleteAgent(theAccount, id)
		}
	}

	// Delete audits
	for _, auditId := range theAccount.AuditIds {
		if id, ok := auditId.(bson.ObjectID); ok {
			_ = DeleteAccountAudit(theAccount, id)
		}
	}

	// Delete integrations
	for _, integrationId := range theAccount.IntegrationIds {
		if id, ok := integrationId.(bson.ObjectID); ok {
			_ = DeleteAccountIntegration(theAccount, id)
		}
	}

	// Unlink from all users
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

	return AccountModel.DeleteById(oid)
}

// ---- Agents ----

func AdminGetAgent(agentId string) (*models.AgentSchema, error) {
	if agentId == "" {
		return nil, errors.New("missing agent_id")
	}

	oid, err := bson.ObjectIDFromHex(agentId)
	if err != nil {
		return nil, fmt.Errorf("invalid agent_id")
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return nil, err
	}

	theAgent := &models.AgentSchema{}
	if err := AgentModel.FindOneById(theAgent, oid); err != nil {
		return nil, err
	}

	return theAgent, nil
}

func AdminListAgents(page, pageSize int32, search string) ([]models.AgentSchema, int, error) {
	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return nil, 0, err
	}

	agents := make([]models.AgentSchema, 0)
	filter := bson.M{}
	if search != "" {
		filter = bson.M{"$or": bson.A{
			bson.M{"agentName": bson.M{"$regex": search, "$options": "i"}},
			bson.M{"apiKey": bson.M{"$regex": search, "$options": "i"}},
		}}
	}

	if err := AgentModel.FindAll(&agents, filter); err != nil {
		return nil, 0, err
	}

	p, ps := normalizePaging(page, pageSize)
	paged, total := paginateSlice(agents, p, ps)
	return paged, total, nil
}

func AdminUpdateAgent(agentId, agentName, apiKey string) (*models.AgentSchema, error) {
	if agentId == "" {
		return nil, errors.New("missing agent_id")
	}

	oid, err := bson.ObjectIDFromHex(agentId)
	if err != nil {
		return nil, fmt.Errorf("invalid agent_id")
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return nil, err
	}

	theAgent := &models.AgentSchema{}
	if err := AgentModel.FindOneById(theAgent, oid); err != nil {
		return nil, err
	}

	update := bson.M{}
	if agentName != "" && agentName != theAgent.AgentName {
		theAgent.AgentName = agentName
		update["agentName"] = agentName
	}
	if apiKey != "" && apiKey != theAgent.APIKey {
		theAgent.APIKey = apiKey
		update["apiKey"] = apiKey
	}
	if len(update) == 0 {
		return theAgent, nil
	}

	theAgent.UpdatedAt = time.Now()
	update["updatedAt"] = theAgent.UpdatedAt

	if err := AgentModel.UpdateData(theAgent, update); err != nil {
		return nil, err
	}

	return theAgent, nil
}

func AdminDeleteAgent(agentId string) error {
	if agentId == "" {
		return errors.New("missing agent_id")
	}

	oid, err := bson.ObjectIDFromHex(agentId)
	if err != nil {
		return fmt.Errorf("invalid agent_id")
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	// Unlink from all accounts that reference this agent
	accounts := make([]models.AccountSchema, 0)
	_ = AccountModel.FindAll(&accounts, bson.M{"agents": bson.M{"$in": bson.A{oid}}})
	for i := range accounts {
		a := &accounts[i]
		newAgents := make(bson.A, 0)
		for _, aId := range a.AgentIds {
			if aOid, ok := aId.(bson.ObjectID); ok {
				if aOid.Hex() != oid.Hex() {
					newAgents = append(newAgents, aOid)
				}
			}
		}
		a.AgentIds = newAgents
		_ = AccountModel.UpdateData(a, bson.M{"agents": a.AgentIds, "updatedAt": time.Now()})
	}

	// Delete agent record (whether linked or not)
	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}
	return AgentModel.DeleteById(oid)
}

// ---- Relationships ----

func AdminAddUserToAccount(userId, accountId string, setActive bool) error {
	if userId == "" || accountId == "" {
		return errors.New("missing user_id or account_id")
	}

	uOid, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return fmt.Errorf("invalid user_id")
	}
	aOid, err := bson.ObjectIDFromHex(accountId)
	if err != nil {
		return fmt.Errorf("invalid account_id")
	}

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return err
	}
	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	theUser := &models.UserSchema{}
	if err := UserModel.FindOneById(theUser, uOid); err != nil {
		return err
	}

	theAccount := &models.AccountSchema{}
	if err := AccountModel.FindOneById(theAccount, aOid); err != nil {
		return err
	}

	for _, existing := range theUser.LinkedAccountIds {
		if eId, ok := existing.(bson.ObjectID); ok {
			if eId.Hex() == aOid.Hex() {
				// Already linked
				if setActive && (theUser.ActiveAccountId.IsZero() || theUser.ActiveAccountId.Hex() != aOid.Hex()) {
					theUser.ActiveAccountId = aOid
					return UserModel.UpdateData(theUser, bson.M{"activeAccount": theUser.ActiveAccountId, "updatedAt": time.Now()})
				}
				return nil
			}
		}
	}

	theUser.LinkedAccountIds = append(theUser.LinkedAccountIds, aOid)
	if setActive || theUser.ActiveAccountId.IsZero() {
		theUser.ActiveAccountId = aOid
	}

	if err := UserModel.UpdateData(theUser, bson.M{"linkedAccounts": theUser.LinkedAccountIds, "activeAccount": theUser.ActiveAccountId, "updatedAt": time.Now()}); err != nil {
		return err
	}

	return AddAccountAudit(theAccount,
		models.AuditType_UserAddedToAccount,
		fmt.Sprintf("User (%s) was added to the account", theUser.Username),
	)
}

func AdminListUserAccounts(userId string) ([]models.AccountSchema, string, error) {
	if userId == "" {
		return nil, "", errors.New("missing user_id")
	}

	uOid, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, "", fmt.Errorf("invalid user_id")
	}

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return nil, "", err
	}
	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return nil, "", err
	}

	theUser := &models.UserSchema{}
	if err := UserModel.FindOneById(theUser, uOid); err != nil {
		return nil, "", err
	}

	ids := make(bson.A, 0, len(theUser.LinkedAccountIds))
	for _, la := range theUser.LinkedAccountIds {
		if oid, ok := la.(bson.ObjectID); ok {
			ids = append(ids, oid)
		}
	}

	active := ""
	if !theUser.ActiveAccountId.IsZero() {
		active = theUser.ActiveAccountId.Hex()
	}

	if len(ids) == 0 {
		return []models.AccountSchema{}, active, nil
	}

	accounts := make([]models.AccountSchema, 0)
	if err := AccountModel.FindAll(&accounts, bson.M{"_id": bson.M{"$in": ids}}); err != nil {
		return nil, "", err
	}

	return accounts, active, nil
}

func AdminRemoveUserFromAccount(userId, accountId string) error {
	if userId == "" || accountId == "" {
		return errors.New("missing user_id or account_id")
	}

	uOid, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return fmt.Errorf("invalid user_id")
	}
	aOid, err := bson.ObjectIDFromHex(accountId)
	if err != nil {
		return fmt.Errorf("invalid account_id")
	}

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return err
	}
	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	theUser := &models.UserSchema{}
	if err := UserModel.FindOneById(theUser, uOid); err != nil {
		return err
	}

	theAccount := &models.AccountSchema{}
	if err := AccountModel.FindOneById(theAccount, aOid); err != nil {
		return err
	}

	newLinked := make(bson.A, 0, len(theUser.LinkedAccountIds))
	removed := false
	for _, existing := range theUser.LinkedAccountIds {
		if eId, ok := existing.(bson.ObjectID); ok {
			if eId.Hex() == aOid.Hex() {
				removed = true
				continue
			}
			newLinked = append(newLinked, eId)
		}
	}

	if !removed {
		return nil
	}

	theUser.LinkedAccountIds = newLinked
	if theUser.ActiveAccountId.Hex() == aOid.Hex() {
		if len(newLinked) > 0 {
			theUser.ActiveAccountId = newLinked[0].(bson.ObjectID)
		} else {
			theUser.ActiveAccountId = bson.NilObjectID
		}
	}

	if err := UserModel.UpdateData(theUser, bson.M{"linkedAccounts": theUser.LinkedAccountIds, "activeAccount": theUser.ActiveAccountId, "updatedAt": time.Now()}); err != nil {
		return err
	}

	return AddAccountAudit(theAccount,
		models.AuditType_UserRemovedFromAccount,
		fmt.Sprintf("User (%s) was removed from the account", theUser.Username),
	)
}

func AdminSetUserActiveAccount(userId, accountId string) error {
	if userId == "" || accountId == "" {
		return errors.New("missing user_id or account_id")
	}

	uOid, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return fmt.Errorf("invalid user_id")
	}
	aOid, err := bson.ObjectIDFromHex(accountId)
	if err != nil {
		return fmt.Errorf("invalid account_id")
	}

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return err
	}

	theUser := &models.UserSchema{}
	if err := UserModel.FindOneById(theUser, uOid); err != nil {
		return err
	}

	linked := false
	for _, existing := range theUser.LinkedAccountIds {
		if eId, ok := existing.(bson.ObjectID); ok {
			if eId.Hex() == aOid.Hex() {
				linked = true
				break
			}
		}
	}
	if !linked {
		return errors.New("account is not linked")
	}

	if !theUser.ActiveAccountId.IsZero() && theUser.ActiveAccountId.Hex() == aOid.Hex() {
		return nil
	}

	theUser.ActiveAccountId = aOid
	return UserModel.UpdateData(theUser, bson.M{"activeAccount": theUser.ActiveAccountId, "updatedAt": time.Now()})
}
