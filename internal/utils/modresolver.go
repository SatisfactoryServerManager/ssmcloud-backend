package utils

import (
	"context"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	resolver "github.com/satisfactorymodding/ficsit-resolver"
	"go.mongodb.org/mongo-driver/bson"
)

type SSMProvider struct{}

func (p SSMProvider) ModVersionsWithDependencies(_ context.Context, modID string) ([]resolver.ModVersion, error) {

	modVersions := make([]resolver.ModVersion, 0)

	ModsModel, err := repositories.GetMongoClient().GetModel("Mod")
	if err != nil {
		return modVersions, err
	}

	var dbMod models.ModSchema
	if err := ModsModel.FindOne(&dbMod, bson.M{"modReference": modID}); err != nil {
		return modVersions, err
	}

	for _, version := range dbMod.Versions {
		modVersion := resolver.ModVersion{
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
			}

			modVersion.Targets = append(modVersion.Targets, modVersionTarget)
		}

		modVersions = append(modVersions, modVersion)
	}

	return modVersions, nil

}

func (p SSMProvider) GetModName(_ context.Context, modReference string) (*resolver.ModName, error) {

	var modName resolver.ModName

	ModsModel, err := repositories.GetMongoClient().GetModel("Mod")
	if err != nil {
		return &modName, err
	}

	var dbMod models.ModSchema
	if err := ModsModel.FindOne(&dbMod, bson.M{"modReference": modReference}); err != nil {
		return &modName, err
	}

	modName.ID = dbMod.ModID
	modName.Name = dbMod.ModName
	modName.ModReference = dbMod.ModReference

	return &modName, nil

}
