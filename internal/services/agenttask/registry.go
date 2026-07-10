package agenttask

import (
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Assignment is one task pushed down an agent's stream.
type Assignment struct {
	TaskID       string
	Action       string
	Data         string
	LeaseToken   string
	Attempt      int32
	MaxAttempts  int32
	LeaseSeconds int32
}

type streamEntry struct {
	connectionID string
	ch           chan Assignment
}

// Registry tracks which agents have a live task stream on *this* replica.
type Registry struct {
	mu      sync.RWMutex
	streams map[bson.ObjectID]*streamEntry
}

var registry = &Registry{streams: map[bson.ObjectID]*streamEntry{}}

func GetRegistry() *Registry { return registry }

// Add registers a stream and returns the receive channel plus a deregister func.
// The deregister func only removes the entry if connectionID still matches, so a
// slow teardown of an old stream cannot detach a freshly reconnected agent.
func (r *Registry) Add(agentID bson.ObjectID, connectionID string) (<-chan Assignment, func()) {
	ch := make(chan Assignment, 1)

	r.mu.Lock()
	r.streams[agentID] = &streamEntry{connectionID: connectionID, ch: ch}
	r.mu.Unlock()

	remove := func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		if e, ok := r.streams[agentID]; ok && e.connectionID == connectionID {
			delete(r.streams, agentID)
			close(e.ch)
		}
	}

	return ch, remove
}

func (r *Registry) Has(agentID bson.ObjectID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.streams[agentID]
	return ok
}

func (r *Registry) ConnectedAgents() []bson.ObjectID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]bson.ObjectID, 0, len(r.streams))
	for id := range r.streams {
		ids = append(ids, id)
	}
	return ids
}

// send delivers an assignment, reporting false if the agent vanished or its
// buffer is full (which means it already has work in flight).
func (r *Registry) send(agentID bson.ObjectID, a Assignment) bool {
	r.mu.RLock()
	e, ok := r.streams[agentID]
	r.mu.RUnlock()

	if !ok {
		return false
	}

	select {
	case e.ch <- a:
		return true
	default:
		return false
	}
}
