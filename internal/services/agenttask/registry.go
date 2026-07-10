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
	ch chan Assignment
}

// Registry tracks which agents have a live task stream on *this* replica.
type Registry struct {
	mu      sync.RWMutex
	streams map[bson.ObjectID]*streamEntry
}

var registry = &Registry{streams: map[bson.ObjectID]*streamEntry{}}

func GetRegistry() *Registry { return registry }

// Add registers a stream and returns the receive channel plus a deregister func.
//
// The deregister func removes the entry only if it is still *this* entry, using
// pointer identity rather than any id the agent supplies. An agent that
// reconnects before its old server-side stream has torn down would otherwise
// have the old teardown evict and close the new, healthy stream.
func (r *Registry) Add(agentID bson.ObjectID) (<-chan Assignment, func()) {
	entry := &streamEntry{ch: make(chan Assignment, 1)}

	r.mu.Lock()
	r.streams[agentID] = entry
	r.mu.Unlock()

	remove := func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		if r.streams[agentID] == entry {
			delete(r.streams, agentID)
			close(entry.ch)
		}
	}

	return entry.ch, remove
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
//
// The read lock is held across the select. remove() closes the channel under the
// write lock, so releasing early would let a disconnecting agent close it between
// the unlock and the send — a panic, not an error. The select has a default case
// and never blocks, so holding the lock here is bounded.
func (r *Registry) send(agentID bson.ObjectID, a Assignment) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.streams[agentID]
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
