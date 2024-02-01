package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/machinebox/graphql"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	graphqlClient *graphql.Client
)

func InitModService() {

	graphqlClient = graphql.NewClient("https://api.ficsit.app/v2/query")

	uptimeticker := time.NewTicker(10 * time.Minute)

	if err := UpdateModsInDB(); err != nil {
		fmt.Println(err)
	}

	go func() {
		for {
			select {
			case <-uptimeticker.C:
				if err := UpdateModsInDB(); err != nil {
					fmt.Println(err)
				}
			case <-_quit:
				uptimeticker.Stop()
				log.Println("Stopped Process Orders Ticker")
				return
			}
		}
	}()
}

func ShutdownModService() error {

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

func GetAllModsFromAPI() ([]models.Mods, error) {

	allMods := make([]models.Mods, 0)

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

	filteredMods := make([]models.Mods, 0)

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

	fmt.Printf("Found a total (%d) mods but total usable mods (%d) \n", len(allMods), len(filteredMods))

	return filteredMods, nil
}

func GetModsFromAPI(offset int) ([]models.Mods, error) {

	emptyMods := make([]models.Mods, 0)

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
                            version,
                            created_at,
                            link,
                            targets {
                                targetName
                                link
                              },
                            sml_version,
                            dependencies {
                                mod_id
                                condition
                            }
                        }
                    }
                }
            }
    `)

	var graphqlResponse app.FicsitAPI_Response_GetMods
	if err := graphqlClient.Run(context.Background(), graphqlRequest, &graphqlResponse); err != nil {
		return emptyMods, err
	}

	return graphqlResponse.GetMods.Mods, nil
}

func UpdateModsInDB() error {

	apiMods, err := GetAllModsFromAPI()
	if err != nil {
		return err
	}

	for idx := range apiMods {

		apiMod := &apiMods[idx]

		var dbMod models.Mods
		if err := mongoose.FindOne(bson.M{"modReference": apiMod.ModReference}, &dbMod); err != nil {

			if errors.Is(err, mongo.ErrNoDocuments) {
				// create mod if not exist
				apiMod.ID = primitive.NewObjectID()

				mongoose.InsertOne(apiMod)

				continue
			}
		}

		dbUpdate := bson.D{{"$set", bson.D{
			{"versions", apiMod.Versions},
			{"downloads", apiMod.Downloads},
			{"logoUrl", apiMod.LogoURL},
			{"hidden", apiMod.Hidden},
		}}}

		if err := mongoose.UpdateDataByID(&dbMod, dbUpdate); err != nil {
			return err
		}

	}

	return nil
}
