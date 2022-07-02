package checkstore

import (
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/serviceregistration/checks"
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

	// Remove will remove ids from the cache and persistent store.
	Remove(allocID string, ids []structs.CheckID) error

	// Purge results for a specific allocation.
	Purge(allocID string) error
}

type shim struct {
	log hclog.Logger

	db state.StateDB

	lock    sync.RWMutex
	current checks.ClientResults
}

// NewStore creates a new store.
func NewStore(log hclog.Logger, db state.StateDB) Shim {
	return &shim{
		log:     log.Named("check_store"),
		db:      db,
		current: make(checks.ClientResults),
	}
}

func (s *shim) restore() {
	s.lock.Lock()
	defer s.lock.Unlock()

	results, err := s.db.GetCheckResults()
	if err != nil {
		s.log.Error("failed to restore health check results", "error", err)
		return
	}

	for id, m := range results {
		s.current[id] = helper.CopyMap(m)
		netlog.Cyan("restore check map[%s]", id)
	}
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

func (s *shim) Remove(allocID string, ids []structs.CheckID) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// remove from cache
	for _, id := range ids {
		delete(s.current[allocID], id)
	}

	// remove from persistent store
	return s.db.DeleteCheckResults(allocID, ids)
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
