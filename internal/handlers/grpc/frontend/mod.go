package frontend

import (
	"context"
	"errors"
	"math"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	accountsvc "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/account"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agentmod"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/mod"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/user"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	modelsV2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	pbModels "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated/models"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/utils/mapper"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// assertAgentModOwned is the authz boundary for the agentmods collection, which
// is cross-account. A nil mod is a rejection, not an allow: "not found" and "not
// yours" must be indistinguishable to the caller.
func assertAgentModOwned(m *modelsV2.AgentModSchema, accountID bson.ObjectID) error {
	if m == nil || m.AccountID != accountID {
		return status.Error(codes.NotFound, "mod not found")
	}
	return nil
}

// resolveAgentForUser is the same walk the other agent RPCs in this package do:
// user by external id -> their active account -> the agent, but only if that
// account owns it. Every mod RPC takes an agentId from the caller, so this is
// the check that keeps one account out of another's agents.
func resolveAgentForUser(eid, agentID string) (*modelsV2.AgentSchema, *modelsV2.AccountSchema, error) {
	oid, err := bson.ObjectIDFromHex(agentID)
	if err != nil {
		return nil, nil, err
	}

	theUser, err := user.GetUser(bson.ObjectID{}, eid, "", "")
	if err != nil {
		return nil, nil, err
	}

	account, err := accountsvc.GetUserActiveAccount(theUser)
	if err != nil {
		return nil, nil, err
	}

	agents, err := agent.GetUserAccountAgents(account, oid)
	if err != nil {
		return nil, nil, err
	}

	if len(agents) == 0 {
		return nil, nil, errors.New("agent not found")
	}

	return agents[0], account, nil
}

// catalogueMeta pulls the display fields the agentmods rows do not carry: a mod's
// name and logo live in the catalogue, and denormalising them would stale.
func catalogueMeta(refs []string) (map[string]models.ModSchema, error) {
	out := make(map[string]models.ModSchema, len(refs))
	if len(refs) == 0 {
		return out, nil
	}

	ModsModel, err := repositories.GetMongoClient().GetModel("Mod")
	if err != nil {
		return nil, err
	}

	dbMods := make([]models.ModSchema, 0, len(refs))
	if err := ModsModel.FindAll(&dbMods, bson.M{"modReference": bson.M{"$in": refs}}); err != nil {
		return nil, err
	}

	for i := range dbMods {
		out[dbMods[i].ModReference] = dbMods[i]
	}

	return out, nil
}

func mapAgentModToProto(m modelsV2.AgentModSchema, meta map[string]models.ModSchema) *pbModels.AgentMod {
	pbMod := &pbModels.AgentMod{
		ModReference:     m.ModReference,
		DesiredVersion:   m.DesiredVersion,
		InstalledVersion: m.InstalledVersion,
		LatestVersion:    m.LatestVersion,
		Installed:        m.Installed,
		NeedsUpdate:      m.NeedsUpdate,
		Direct:           m.Direct,
		Config:           m.Config,
	}

	if dbMod, ok := meta[m.ModReference]; ok {
		pbMod.ModName = dbMod.ModName
		pbMod.LogoUrl = dbMod.LogoURL
	}

	return pbMod
}

func (s *Handler) GetAgentMods(ctx context.Context, in *pb.GetAgentModsRequest) (*pb.GetAgentModsResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theAgent, _, err := resolveAgentForUser(in.Eid, in.AgentId)
	if err != nil {
		return nil, err
	}

	agentMods, err := agentmod.ListForAgent(theAgent.ID)
	if err != nil {
		return nil, err
	}

	// Build the agent's install-state reference sets for server-side filtering.
	refs := make([]string, 0, len(agentMods))
	installedRefs := make([]string, 0)
	updatableRefs := make([]string, 0)
	for _, m := range agentMods {
		if m.ModReference == "" {
			continue
		}
		refs = append(refs, m.ModReference)
		if m.Installed {
			installedRefs = append(installedRefs, m.ModReference)
		}
		if m.NeedsUpdate {
			updatableRefs = append(updatableRefs, m.ModReference)
		}
	}

	meta, err := catalogueMeta(refs)
	if err != nil {
		return nil, err
	}

	pbAgentMods := make([]*pbModels.AgentMod, 0, len(agentMods))
	for _, m := range agentMods {
		pbAgentMods = append(pbAgentMods, mapAgentModToProto(m, meta))
	}

	modFilter := mod.ModQueryFilter{
		Search:        in.Search,
		ShowAvailable: in.FilterAvailable,
		ShowInstalled: in.FilterInstalled,
		OnlyUpdatable: in.OnlyUpdatable,
		IncludeHidden: in.IncludeHidden,
		InstalledRefs: installedRefs,
		UpdatableRefs: updatableRefs,
	}

	mods, err := mod.GetModsFromDB(int(in.Page), in.Sort, in.Direction, modFilter)
	if err != nil {
		return nil, err
	}
	modCount, err := mod.GetDBModCount(modFilter)
	if err != nil {
		return nil, err
	}

	pages := float64(modCount) / float64(30)

	pbMods := make([]*pbModels.Mod, 0, len(*mods))
	for i := range *mods {
		pbMods = append(pbMods, mapper.MapModToProto(&(*mods)[i]))
	}

	return &pb.GetAgentModsResponse{
		Mods:       pbMods,
		AgentMods:  pbAgentMods,
		TotalCount: modCount,
		Pages:      int32(math.Ceil(pages)),
	}, nil
}

func modChangeFromProto(in *pb.ModChangeRequest) agentmod.ModChange {
	return agentmod.ModChange{
		Op:           in.Op,
		ModReference: in.ModReference,
		Version:      in.Version,
	}
}

func mapChangedMods(in []agentmod.ChangedMod) []*pb.ChangedMod {
	out := make([]*pb.ChangedMod, 0, len(in))
	for _, c := range in {
		out = append(out, &pb.ChangedMod{
			ModReference: c.ModReference,
			From:         c.From,
			To:           c.To,
			Dependency:   c.Dependency,
		})
	}
	return out
}

func (s *Handler) PreviewModChange(ctx context.Context, in *pb.ModChangeRequest) (*pb.PreviewModChangeResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "missing change")
	}

	theAgent, _, err := resolveAgentForUser(in.Eid, in.AgentId)
	if err != nil {
		return nil, err
	}

	change, err := agentmod.Preview(theAgent.ID, modChangeFromProto(in))
	if err != nil {
		return nil, err
	}

	return &pb.PreviewModChangeResponse{
		Added:   mapChangedMods(change.Added),
		Removed: mapChangedMods(change.Removed),
		Changed: mapChangedMods(change.Changed),
		// The client needs this to know whether to offer the apply-now /
		// apply-on-restart choice rather than just applying.
		ServerRunning: theAgent.Status.Running,
	}, nil
}

func (s *Handler) ApplyModChange(ctx context.Context, in *pb.ApplyModChangeRequest) (*pb.ApplyModChangeResponse, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	if in.Change == nil {
		return nil, status.Error(codes.InvalidArgument, "missing change")
	}

	theAgent, account, err := resolveAgentForUser(in.Change.Eid, in.Change.AgentId)
	if err != nil {
		return nil, err
	}

	taskIDs, err := agentmod.Apply(
		theAgent.ID,
		account.ID,
		modChangeFromProto(in.Change),
		in.ApplyNow,
		modelsV2.TaskTrigger{Type: modelsV2.TaskTriggerUser, ExternalID: in.Change.Eid},
	)
	if err != nil {
		return nil, err
	}

	return &pb.ApplyModChangeResponse{TaskIds: taskIDs}, nil
}

func (s *Handler) UpdateAgentModConfigText(ctx context.Context, in *pb.UpdateAgentModConfigTextRequest) (*pbModels.SSMEmpty, error) {
	if err := s.validateAPIKey(ctx); err != nil {
		return nil, err
	}

	theAgent, account, err := resolveAgentForUser(in.Eid, in.AgentId)
	if err != nil {
		return nil, err
	}

	theMod, err := agentmod.Get(theAgent.ID, in.ModReference)
	if err != nil {
		return nil, err
	}

	if err := assertAgentModOwned(theMod, account.ID); err != nil {
		return nil, err
	}

	// ApplyConfigOnly performs the SetConfig and enqueues the sync: a config-text
	// change moves no versions, so Apply would short-circuit on the empty diff.
	if _, err := agentmod.ApplyConfigOnly(
		theAgent.ID,
		account.ID,
		in.ModReference,
		in.Config,
		false,
		modelsV2.TaskTrigger{Type: modelsV2.TaskTriggerUser, ExternalID: in.Eid},
	); err != nil {
		return nil, err
	}

	return &pbModels.SSMEmpty{}, nil
}
