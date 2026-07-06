package admin

import (
	"errors"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/user"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

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

	return user.GetUser(oid, externalId, email, "")
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
