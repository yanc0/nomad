package checkstore

import (
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/exp/slices"
	"gophers.dev/pkgs/netlog"
)

// A Shim is used to track the latest check status information, one layer above
// the client persistent store so we can do efficient indexing, etc.
type Shim interface {
	// Set the latest result for a specific check.
	Set(allocID string, result *structs.CheckQueryResult) error

	// List the latest results for a specific allocation.
	List(allocID string) map[structs.CheckID]*structs.CheckQueryResult

	// Unwanted returns the set of IDs being stored that are not in ids.
	Unwanted(allocID string, ids []structs.CheckID) []structs.CheckID

	// Keep will reconcile the current set of stored check results with the
	// list of checkIDs for check results that should be kept.
	Keep(allocID string, ids []structs.CheckID) error // todo: opimize, we have the list to remove now

	// Purge results for a specific allocation.
	Purge(allocID string) error
}

// AllocResultMap is a view of the check_id -> latest result for group and task
// checks in an allocation.
type AllocResultMap map[structs.CheckID]*structs.CheckQueryResult

// diff returns the set of IDs in ids that are not in m.
func (m AllocResultMap) diff(ids []structs.CheckID) []structs.CheckID {
	netlog.Red("ARM m: %v, ids: %v", m, ids)
	var missing []structs.CheckID
	for _, id := range ids {
		if _, exists := m[id]; !exists {
			missing = append(missing, id)
		}
	}
	return missing
}

// ClientResultMap is a holistic view of alloc_id -> check_id -> latest result
// group and task checks across all allocations on a client.
type ClientResultMap map[string]AllocResultMap

type shim struct {
	log hclog.Logger

	db state.StateDB

	lock    sync.RWMutex
	current ClientResultMap
}

// NewStore creates a new store.
//
// (todo: and will initialize from db)
func NewStore(log hclog.Logger, db state.StateDB) Shim {
	return &shim{
		log:     log.Named("check_store"),
		db:      db,
		current: make(ClientResultMap),
	}
}

func (s *shim) restore() {
	// todo restore state from db
	netlog.Red("shim.restore not yet implemented")
}

func (s *shim) Set(allocID string, qr *structs.CheckQueryResult) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	netlog.Red("Set id: %s, result: %v", allocID, qr)

	if allocID == "" {
		panic("empty alloc id")
	}

	if qr.ID == "" {
		panic("empty qr id")
	}

	s.log.Trace("setting check status", "alloc_id", allocID, "check_id", qr.ID, "status", qr.Status)

	if _, exists := s.current[allocID]; !exists {
		s.current[allocID] = make(map[structs.CheckID]*structs.CheckQueryResult)
	}

	s.current[allocID][qr.ID] = qr

	return s.db.PutCheckResult(allocID, qr)
}

func (s *shim) List(allocID string) map[structs.CheckID]*structs.CheckQueryResult {
	s.lock.RLock()
	defer s.lock.RUnlock()

	m, exists := s.current[allocID]
	if !exists {
		return nil
	}

	return helper.CopyMap(m)
}

func (s *shim) Purge(allocID string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// remove from our map
	delete(s.current, allocID)

	// remove from persistent store
	return s.db.PurgeCheckResults(allocID)
}

func (s *shim) Keep(allocID string, ids []structs.CheckID) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// remove each id in ids from the cache & persistence store
	var remove []structs.CheckID
	for id := range s.current[allocID] {
		if !slices.Contains(ids, id) {
			delete(s.current[allocID], id)
			remove = append(remove, id)
		}
	}

	// remove from persistent store
	return s.db.DeleteCheckResults(allocID, remove)
}

func (s *shim) Unwanted(allocID string, ids []structs.CheckID) []structs.CheckID {
	s.lock.Lock()
	defer s.lock.Unlock()

	var remove []structs.CheckID
	for id := range s.current[allocID] {
		if !slices.Contains(ids, id) {
			remove = append(remove, id)
		}
	}

	return remove
}
