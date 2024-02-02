package services

import (
	"context"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/machinebox/graphql"
	"github.com/mrhid6/go-mongoose/mongoose"
	resolver "github.com/satisfactorymodding/ficsit-resolver"
	"go.mongodb.org/mongo-driver/bson"
)

type SSMProvider struct{}

func (p SSMProvider) SMLVersions(_ context.Context) ([]resolver.SMLVersion, error) {

	graphqlClient = graphql.NewClient("https://api.ficsit.app/v2/query")
	smlVersions := make([]resolver.SMLVersion, 0)

	graphqlRequest := graphql.NewRequest(`
	{
  getSMLVersions{
    sml_versions {
      id,
      version,
      satisfactory_version
      targets{
        targetName,
        link
      }
    }
  }
}
`)

	var graphqlResponse app.FicsitAPI_Response_GetSMLVersions
	if err := graphqlClient.Run(context.Background(), graphqlRequest, &graphqlResponse); err != nil {
		return smlVersions, err
	}

	return graphqlResponse.GetSMLVersions.SMLVersions, nil

}

func (p SSMProvider) ModVersionsWithDependencies(_ context.Context, modID string) ([]resolver.ModVersion, error) {

	modVersions := make([]resolver.ModVersion, 0)

	var dbMod models.Mods
	if err := mongoose.FindOne(bson.M{"modReference": modID}, &dbMod); err != nil {
		return modVersions, err
	}

	for _, version := range dbMod.Versions {
		modVersion := resolver.ModVersion{
			ID:           version.ID,
			Version:      version.Version,
			Dependencies: make([]resolver.Dependency, 0),
			Targets:      make([]resolver.Target, 0),
		}

		for _, dep := range version.Dependencies {
			modVersionDep := resolver.Dependency{
				ModID:     dep.ModReference,
				Condition: dep.Condition,
				Optional:  dep.Optional,
			}

			modVersion.Dependencies = append(modVersion.Dependencies, modVersionDep)
		}

		for _, target := range version.Targets {
			modVersionTarget := resolver.Target{
				TargetName: resolver.TargetName(target.TargetName),
				Hash:       target.Hash,
				Size:       target.Size,
				VersionID:  target.VersionID,
			}

			modVersion.Targets = append(modVersion.Targets, modVersionTarget)
		}

		modVersions = append(modVersions, modVersion)
	}

	return modVersions, nil

}

func (p SSMProvider) GetModName(_ context.Context, modReference string) (*resolver.ModName, error) {

	var modName resolver.ModName

	var dbMod models.Mods
	if err := mongoose.FindOne(bson.M{"modReference": modReference}, &dbMod); err != nil {
		return &modName, err
	}

	modName.ID = dbMod.ModID
	modName.Name = dbMod.ModName
	modName.ModReference = dbMod.ModReference

	return &modName, nil

}
