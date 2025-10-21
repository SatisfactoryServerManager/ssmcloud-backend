package v2

import (
	"fmt"

	"github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetMods(page int, sort string, direction string, search string) (*[]models.Mods, error) {

	modsRes := make([]models.Mods, 0)

	// Determine sorting field
	sortField := "modName"
	if sort == "downloads" {
		sortField = "downloads"
	}

	// Determine sort direction
	sortDir := 1 // ascending
	if direction == "desc" {
		sortDir = -1
	}

	// Build filter (supports partial match on "name")
	filter := bson.M{}
	if search != "" {
		filter["modName"] = bson.M{
			"$regex":   search,
			"$options": "i", // case-insensitive
		}
	}

	// Pagination
	skip := int64(page * 30)
	limit := int64(30)

	findOptions := options.Find().
		SetSort(bson.D{{Key: sortField, Value: sortDir}}).
		SetSkip(skip).
		SetLimit(limit)

	if err := mongoose.FindAllWithOptions(filter, *findOptions, &modsRes); err != nil {
		return nil, fmt.Errorf("error getting mods from db with error: %s", err.Error())
	}

	return &modsRes, nil
}

func GetModCount() (int64, error) {
	count, err := mongoose.CountDocuments("mods", bson.M{})
	if err != nil {
		return 0, fmt.Errorf("error getting mod count with error: %s", err.Error())
	}

	return count, nil
}
