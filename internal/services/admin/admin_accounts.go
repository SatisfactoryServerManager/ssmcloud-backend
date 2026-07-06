package admin

import (
	"errors"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/account"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

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

	theAccount := &models.AccountSchema{}
	if err := AccountModel.FindOneById(theAccount, oid); err != nil {
		return err
	}

	return account.PurgeAccount(theAccount)
}
