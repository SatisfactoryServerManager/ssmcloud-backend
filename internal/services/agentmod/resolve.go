package agentmod

import (
	"fmt"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	resolver "github.com/satisfactorymodding/ficsit-resolver"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ficsitBaseURL is where a mod version target's relative link hangs off. The
// agent is given absolute URLs so it never has to know this.
const ficsitBaseURL = "https://api.ficsit.app"

// ChangedMod is one line of a change, as the confirmation dialog renders it.
type ChangedMod struct {
	ModReference string `json:"modReference"`
	From         string `json:"from,omitempty"`
	To           string `json:"to,omitempty"`
	// Dependency is true when the resolver pulled this mod in rather than the user
	// choosing it. "Also installs: A, B, C".
	Dependency bool `json:"dependency"`
}

type Change struct {
	Added   []ChangedMod `json:"added"`
	Removed []ChangedMod `json:"removed"`
	Changed []ChangedMod `json:"changed"`
}

func (c Change) IsEmpty() bool {
	return len(c.Added) == 0 && len(c.Removed) == 0 && len(c.Changed) == 0
}

// Diff compares the agent's current selection against a freshly resolved
// lockfile. It is pure, and it is what both Preview (which renders it) and Apply
// (which acts on it) are built from.
func Diff(current []v2.AgentModSchema, next v2.Lockfile) Change {
	c := Change{
		Added:   make([]ChangedMod, 0),
		Removed: make([]ChangedMod, 0),
		Changed: make([]ChangedMod, 0),
	}

	byRef := make(map[string]v2.AgentModSchema, len(current))
	for _, m := range current {
		byRef[m.ModReference] = m
	}

	inNext := make(map[string]bool, len(next.Mods))

	for _, lock := range next.Mods {
		inNext[lock.ModReference] = true

		existing, ok := byRef[lock.ModReference]
		if !ok {
			c.Added = append(c.Added, ChangedMod{
				ModReference: lock.ModReference,
				To:           lock.Version,
				Dependency:   !lock.Direct,
			})
			continue
		}

		if existing.DesiredVersion != lock.Version {
			c.Changed = append(c.Changed, ChangedMod{
				ModReference: lock.ModReference,
				From:         existing.DesiredVersion,
				To:           lock.Version,
				Dependency:   !lock.Direct,
			})
		}
	}

	// A shared dependency (e.g. Ficsit above) is still in next.Mods because the
	// resolver re-derived it for the surviving direct mod, so it is caught by the
	// inNext check above and never lands here — it is never invented as a removal.
	for _, m := range current {
		if !inNext[m.ModReference] {
			c.Removed = append(c.Removed, ChangedMod{
				ModReference: m.ModReference,
				From:         m.DesiredVersion,
				Dependency:   !m.Direct,
			})
		}
	}

	return c
}

// Resolve pins the agent's current direct selection.
func Resolve(agentID bson.ObjectID) (v2.Lockfile, error) {
	mods, err := ListForAgent(agentID)
	if err != nil {
		return v2.Lockfile{}, err
	}

	direct := make(map[string]string)
	for _, m := range mods {
		if m.Direct {
			direct[m.ModReference] = m.DesiredVersion
		}
	}

	return ResolveSelection(agentID, direct)
}

// ResolveSelection pins a hypothetical direct selection: modReference -> the
// version constraint the user pinned, or "" for "latest compatible".
//
// This is the one place the dependency graph is computed. The agent never
// resolves anything, so two agents cannot disagree, and an impossible selection
// is an error the user sees at click time rather than a task that dies later.
func ResolveSelection(agentID bson.ObjectID, direct map[string]string) (v2.Lockfile, error) {
	lf := v2.Lockfile{Mods: make([]v2.ModLock, 0)}

	platform, err := agentPlatform(agentID)
	if err != nil {
		return v2.Lockfile{}, err
	}
	// Never guess: shipping a Linux build to a Windows agent (or vice versa)
	// produces a mod that silently fails to load, with no error anywhere.
	if platform == "" {
		return v2.Lockfile{}, fmt.Errorf("agent has not reported its platform yet")
	}

	if len(direct) == 0 {
		return lf, nil
	}

	// ficsit-resolver's Constraints are plain semver range strings, keyed by mod
	// reference; there is no dedicated Constraints type in the real API.
	constraints := make(map[string]string, len(direct))
	for ref, version := range direct {
		if version == "" {
			constraints[ref] = ">=0.0.0"
		} else {
			constraints[ref] = "=" + version
		}
	}

	r := resolver.NewDependencyResolver(utils.SSMProvider{})

	// requiredTargets prunes any mod version lacking a build for this agent's
	// platform out of the candidate set entirely, so the solver never picks a
	// version it would then have nothing to install.
	requiredTargets := []resolver.TargetName{resolver.TargetName(platform)}

	resolved, err := r.ResolveModDependencies(constraints, nil, 0, requiredTargets)
	if err != nil {
		return lf, fmt.Errorf("cannot resolve mods: %w", err)
	}

	// The resolver gives references and versions. The download URL, hash, and size
	// come from the catalogue, and the .cfg text from the agent's own row.
	for ref, resolvedMod := range resolved.Mods {
		lock, err := lockFor(agentID, ref, resolvedMod.Version, platform)
		if err != nil {
			return v2.Lockfile{}, err
		}

		_, isDirect := direct[ref]
		lock.Direct = isDirect

		lf.Mods = append(lf.Mods, lock)
	}

	return lf, nil
}

// agentPlatform reads the agent's reported mod target ("WindowsServer" or
// "LinuxServer") straight from the agent document. It is queried directly,
// the same way lockFor queries the catalogue directly, rather than through the
// agent package, to avoid a service-to-service dependency for one field.
func agentPlatform(agentID bson.ObjectID) (string, error) {
	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return "", err
	}

	dbAgent := &v2.AgentSchema{}
	if err := AgentModel.FindOne(dbAgent, bson.M{"_id": agentID}); err != nil {
		return "", fmt.Errorf("cannot load agent %s: %w", agentID.Hex(), err)
	}

	return dbAgent.Config.Platform, nil
}

// lockFor pins one mod: catalogue lookup for the artifact, agentmods lookup for
// the config text.
func lockFor(agentID bson.ObjectID, modReference, version, platform string) (v2.ModLock, error) {
	lock := v2.ModLock{ModReference: modReference, Version: version}

	ModsModel, err := repositories.GetMongoClient().GetModel("Mod")
	if err != nil {
		return lock, err
	}

	dbMod := &models.ModSchema{}
	if err := ModsModel.FindOne(dbMod, bson.M{"modReference": modReference}); err != nil {
		return lock, fmt.Errorf("mod %s is not in the catalogue: %w", modReference, err)
	}

	target, err := serverTarget(dbMod, version, platform)
	if err != nil {
		return lock, err
	}

	lock.DownloadURL = ficsitBaseURL + target.Link
	lock.Hash = target.Hash
	lock.Size = target.Size

	existing, err := Get(agentID, modReference)
	if err != nil {
		return lock, err
	}
	if existing != nil {
		lock.Config = existing.Config
	}
	if lock.Config == "" {
		lock.Config = "{}"
	}

	return lock, nil
}

// serverTarget finds the build of a mod version for the agent's platform.
//
// A mod with no build for that platform cannot be installed on that agent, so
// this is an error rather than a skip: the old code skipped it silently and the
// user never learned why the mod never appeared.
func serverTarget(dbMod *models.ModSchema, version, platform string) (models.ModVersionTarget, error) {
	if platform == "" {
		return models.ModVersionTarget{}, fmt.Errorf("agent has not reported its platform yet")
	}

	for _, mv := range dbMod.Versions {
		if mv.Version != version {
			continue
		}
		for _, t := range mv.Targets {
			if t.TargetName == platform {
				return t, nil
			}
		}
		return models.ModVersionTarget{}, fmt.Errorf("mod %s %s has no %s build", dbMod.ModReference, version, platform)
	}
	return models.ModVersionTarget{}, fmt.Errorf("mod %s has no version %s in the catalogue", dbMod.ModReference, version)
}
