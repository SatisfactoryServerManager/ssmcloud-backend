package v2

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/types"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/logger"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	"github.com/machinebox/graphql"
	"github.com/mrhid6/go-mongoose-lock/joblock"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	graphqlClient *graphql.Client
	updateModsJob *joblock.JobLockTask
)

func InitModService() {

	graphqlClient = graphql.NewClient("https://api.ficsit.app/v2/query")

	updateModsJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"updateModsJob", func() {
			if err := UpdateModsInDB(); err != nil {
				fmt.Println(err)
			}
		},
		5*time.Minute,
		1*time.Minute,
		false,
	)

	ctx := context.Background()
	if err := updateModsJob.Run(ctx); err != nil {
		logger.GetErrorLogger().Printf("%+v\n", err)
	}

	logger.GetDebugLogger().Println("Initalized Mod Service")
}

func ShutdownModService() error {
	updateModsJob.UnLock(context.TODO())
	logger.GetDebugLogger().Println("Shutdown Mod Service")
	return nil
}

func GetModCountFromAPI() (int, error) {
	graphqlRequest := graphql.NewRequest(`
        {
            getMods(filter: {
                hidden: true
            }) {
                count
            }
        }
    `)
	var graphqlResponse map[string]map[string]int
	if err := graphqlClient.Run(context.Background(), graphqlRequest, &graphqlResponse); err != nil {
		return 0, err
	}
	count := graphqlResponse["getMods"]["count"]

	return count, nil
}

func GetAllModsFromAPI() ([]models.ModSchema, error) {

	allMods := make([]models.ModSchema, 0)

	count, err := GetModCountFromAPI()
	if err != nil {
		return allMods, err
	}

	for i := 0; i < int(math.Ceil(float64(count)/100)); i++ {
		startOffset := i * 100

		mods, err := GetModsFromAPI(startOffset)

		if err != nil {
			fmt.Println(err)
		}

		allMods = append(allMods, mods...)
	}

	filteredMods := make([]models.ModSchema, 0)

	for _, mod := range allMods {

		if len(mod.Versions) == 0 {
			continue
		}

		filteredModVersions := make([]models.ModVersion, 0)

		for _, modVersion := range mod.Versions {
			if len(modVersion.Targets) == 0 {
				continue
			}

			canAddVersion := false

			for _, modVersionTarget := range modVersion.Targets {
				if modVersionTarget.TargetName == "LinuxServer" {
					canAddVersion = true
				}

				if modVersionTarget.TargetName == "WindowsServer" {
					canAddVersion = true
				}
			}

			if canAddVersion {
				filteredModVersions = append(filteredModVersions, modVersion)
			}
		}

		mod.Versions = filteredModVersions

		if len(mod.Versions) == 0 {
			continue
		}

		filteredMods = append(filteredMods, mod)
	}

	logger.GetDebugLogger().Printf("Found a total (%d) mods but total usable mods (%d) \n", len(allMods), len(filteredMods))

	return filteredMods, nil
}

func GetModsFromAPI(offset int) ([]models.ModSchema, error) {

	emptyMods := make([]models.ModSchema, 0)

	graphqlRequest := graphql.NewRequest(`
        {
                getMods(filter: {
                    limit: 100,
                    offset: ` + strconv.Itoa(offset) + `,
                    hidden: true
                }) {
                    mods {
                        id,
                        name,
                        hidden,
                        logo,
                        mod_reference,
                        downloads,
                        versions {
							id,
                            version,
                            created_at,
                            link,
                            targets {
								VersionID
								hash
                                targetName
								size
                                link
                              },
                            dependencies {
                                mod_id
                                condition
								optional
                            }
                        }
                    }
                }
            }
    `)

	var graphqlResponse types.FicsitAPI_Response_GetMods
	if err := graphqlClient.Run(context.Background(), graphqlRequest, &graphqlResponse); err != nil {
		return emptyMods, err
	}

	return graphqlResponse.GetMods.Mods, nil
}

func UpdateModsInDB() error {

	ModsModel, err := repositories.GetMongoClient().GetModel("Mod")
	if err != nil {
		return err
	}

	logger.GetDebugLogger().Println("Running Update Mods Job")
	apiMods, err := GetAllModsFromAPI()
	if err != nil {
		return err
	}

	for idx := range apiMods {

		apiMod := &apiMods[idx]

		dbMod := &models.ModSchema{}
		if err := ModsModel.FindOne(dbMod, bson.M{"modReference": apiMod.ModReference}); err != nil {

			if errors.Is(err, mongo.ErrNoDocuments) {
				// create mod if not exist
				apiMod.ID = primitive.NewObjectID()

				ModsModel.Create(apiMod)

				continue
			}
		}

		dbUpdate := bson.M{
			"versions":  apiMod.Versions,
			"downloads": apiMod.Downloads,
			"logoUrl":   apiMod.LogoURL,
			"hidden":    apiMod.Hidden,
		}

		if err := ModsModel.UpdateData(dbMod, dbUpdate); err != nil {
			return err
		}

	}

	logger.GetDebugLogger().Println("Finished Update Mods Job")

	return nil
}

func GetModsFromDB(page int, sort string, direction string, search string) (*[]models.ModSchema, error) {

	ModsModel, err := repositories.GetMongoClient().GetModel("Mod")
	if err != nil {
		return nil, err
	}

	modsRes := make([]models.ModSchema, 0)

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

	if err := ModsModel.FindAllWithOptions(&modsRes, filter, findOptions); err != nil {
		return nil, fmt.Errorf("error getting mods from db with error: %s", err.Error())
	}

	return &modsRes, nil
}

func GetDBModCount() (int64, error) {
	count, err := mongoose.CountDocuments("mods", bson.M{})
	if err != nil {
		return 0, fmt.Errorf("error getting mod count with error: %s", err.Error())
	}

	return count, nil
}
