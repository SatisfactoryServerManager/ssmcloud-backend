package admin

import (
	"errors"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/audit"
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

	return audit.AddAccountAudit(theAccount,
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

	return audit.AddAccountAudit(theAccount,
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
