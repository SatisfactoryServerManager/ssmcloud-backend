package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Mods struct {
	ID           primitive.ObjectID `json:"_id" bson:"_id"`
	ModID        string             `json:"id" bson:"modId"`
	ModName      string             `json:"name" bson:"modName"`
	ModReference string             `json:"mod_reference" bson:"modReference"`
	Hidden       bool               `json:"hidden" bson:"hidden"`
	LogoURL      string             `json:"logo" bson:"logoUrl"`
	Downloads    int                `json:"downloads" bson:"downloads"`
	Versions     []ModVersion       `json:"versions" bson:"versions"`
}

type ModVersion struct {
	ID           string                 `json:"id" bson:"id"`
	Version      string                 `json:"version" bson:"version"`
	CreatedAt    string                 `json:"created_at" bson:"created_at"`
	SMLVersion   string                 `json:"sml_version" bson:"sml_version"`
	Targets      []ModVersionTarget     `json:"targets" bson:"targets"`
	Dependencies []ModVersionDependency `json:"dependencies" bson:"dependencies"`
}

type ModVersionTarget struct {
	TargetName string `json:"targetName" bson:"targetName"`
	Link       string `json:"link" bson:"link"`
	Hash       string `json:"hash" bson:"hash"`
	Size       int64  `json:"size" bson:"size"`
	VersionID  string `json:"VersionID" bson:"VersionID"`
}

type ModVersionDependency struct {
	ModReference string `json:"mod_id" bson:"modReference"`
	Condition    string `json:"condition" bson:"condition"`
	Optional     bool   `json:"optional" bson:"optional"`
}
