package frontend

import (
	"testing"

	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestAssertAgentModOwnedRejectsAnotherAccount(t *testing.T) {
	mine := bson.NewObjectID()
	theirs := bson.NewObjectID()

	err := assertAgentModOwned(&v2.AgentModSchema{AccountID: theirs}, mine)

	if err == nil {
		t.Fatal("expected a mod belonging to another account to be rejected")
	}
}

func TestAssertAgentModOwnedAllowsOwnAccount(t *testing.T) {
	mine := bson.NewObjectID()

	if err := assertAgentModOwned(&v2.AgentModSchema{AccountID: mine}, mine); err != nil {
		t.Fatalf("expected the owning account to be allowed, got %s", err)
	}
}

func TestAssertAgentModOwnedRejectsAMissingMod(t *testing.T) {
	if err := assertAgentModOwned(nil, bson.NewObjectID()); err == nil {
		t.Fatal("expected a missing mod to be rejected rather than treated as owned")
	}
}
