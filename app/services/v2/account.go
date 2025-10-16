package v2

import (
	"errors"
	"fmt"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
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

func GetMyUserAccount(theUser *models.UserSchema) (*models.AccountSchema, error) {

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return nil, err
	}

	if theUser.ActiveAccountId.IsZero() {
		return nil, nil
	}

	if err := UserModel.PopulateField(theUser, "ActiveAccount"); err != nil {
		return nil, fmt.Errorf("error populating active account with error: %s", err.Error())
	}

	return &theUser.ActiveAccount, nil
}

func GetMyUserLinkedAccounts(theUser *models.UserSchema) (*[]models.AccountSchema, error) {

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return nil, err
	}

	if len(theUser.LinkedAccountIds) == 0 {
		emptyAccounts := make([]models.AccountSchema, 0)
		return &emptyAccounts, nil
	}

	if err := UserModel.PopulateField(theUser, "LinkedAccounts"); err != nil {
		return nil, fmt.Errorf("error populating linked accounts with error: %s", err.Error())
	}

	return &theUser.LinkedAccounts, nil
}
