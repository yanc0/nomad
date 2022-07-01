package allocrunner

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/serviceregistration/checks"
	"github.com/hashicorp/nomad/client/serviceregistration/checks/checkstore"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"gophers.dev/pkgs/netlog"
)

const (
	// checksHookName is the name of this hook as appears in logs
	checksHookName = "checks_hook"
)

// observers maintains a map from check_id -> observer for a particular check. Each
// observer in the map must share the same context.
type observers map[structs.CheckID]*observer

// An observer is used to execute a particular check on its interval and update the
// check store with those results.
type observer struct {
	ctx     context.Context
	cancel  context.CancelFunc
	shim    checkstore.Shim
	checker checks.Checker

	allocID string
	check   *structs.ServiceCheck
	qc      *checks.QueryContext
}

func (o *observer) start() {
	// compromise between immediate (too early) and waiting full interval (slow)
	firstWait := o.check.Interval / 2

	timer, cancel := helper.NewSafeTimer(firstWait)
	defer cancel()

	netlog.Cyan("observer started for check: %s", o.check.Name)

	for {
		select {
		case <-o.ctx.Done():
			netlog.Cyan("observer exit, check: %s", o.check.Name)
			return
		case <-timer.C:

			// execute the check
			query := checks.GetCheckQuery(o.check)
			result := o.checker.Do(o.qc, query)

			netlog.Cyan("observer result: %s ...", result)
			netlog.Cyan("%s", result.Output)

			// and put the results into the store
			_ = o.shim.Set(o.allocID, result)

			timer.Reset(o.check.Interval)
		}
	}
}

func (o *observer) stop() {
	o.cancel()
}

// checksHook manages checks of Nomad service registrations, at both the group and
// task level, by storing / removing them from the Client state store.
//
// Does not manage Consul service checks; see groupServiceHook instead.
type checksHook struct {
	logger  hclog.Logger
	network structs.NetworkStatus
	shim    checkstore.Shim
	checker checks.Checker
	allocID string

	// fields that get re-initialized on allocation update
	lock      sync.RWMutex
	ctx       context.Context
	stop      func()
	observers observers
	alloc     *structs.Allocation
}

func newChecksHook(
	logger hclog.Logger,
	alloc *structs.Allocation,
	shim checkstore.Shim,
	network structs.NetworkStatus,
) *checksHook {
	h := &checksHook{
		logger:  logger.Named(checksHookName),
		allocID: alloc.ID,
		alloc:   alloc,
		shim:    shim,
		network: network,
		checker: checks.New(logger, alloc),
	}
	h.initialize(alloc)
	return h
}

// initialize the dynamic fields of checksHook, which is to say setup all the
// observers and query context things associated with the alloc.
//
// Should be called during initial setup only.
func (h *checksHook) initialize(alloc *structs.Allocation) {
	h.lock.Lock()
	defer h.lock.Unlock()

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		return
	}

	// fresh context and stop function for this allocation
	h.ctx, h.stop = context.WithCancel(context.Background())

	// fresh set of observers
	h.observers = make(observers)

	// set the current alloc
	h.alloc = alloc
}

// observe will create the observer for each service in services.
// services must use only nomad service provider.
//
// Caller must hold h.lock.
func (h *checksHook) observe(alloc *structs.Allocation, services []*structs.Service) {
	var ports structs.AllocatedPorts
	var networks structs.Networks
	if alloc.AllocatedResources != nil {
		ports = alloc.AllocatedResources.Shared.Ports
		networks = alloc.AllocatedResources.Shared.Networks
	}

	for _, service := range services {
		for _, check := range service.Checks {

			// remember the initialization time
			now := time.Now().UTC().Unix()

			// create the deterministic check id for this check
			id := structs.MakeCheckID(alloc.ID, alloc.TaskGroup, check.TaskName, check.Name)

			netlog.Purple("observe %s : %s : %s", service.Name, check.Name, id)

			// an observer for this check already exists
			if _, exists := h.observers[id]; exists {
				netlog.Purple(" -> already exists")
				continue
			}

			ctx, cancel := context.WithCancel(h.ctx)

			// create the observer for this check
			h.observers[id] = &observer{
				ctx:     ctx,
				cancel:  cancel,
				check:   check.Copy(),
				shim:    h.shim,
				checker: h.checker,
				allocID: h.allocID,
				qc: &checks.QueryContext{
					ID:               id,
					CustomAddress:    service.Address,
					ServicePortLabel: service.PortLabel,
					Ports:            ports,
					Networks:         networks,
					NetworkStatus:    h.network,
				},
			}

			netlog.Purple(" -> create observer")

			// insert a pending result into state store for each check
			result := checks.Stub(id, structs.GetCheckMode(check), now)
			if err := h.shim.Set(h.allocID, result); err != nil {
				h.logger.Error("failed to set initial check status", "id", h.allocID, "error", err)
				continue
			}

			// start the observer
			go h.observers[id].start()
		}
	}
}

func (h *checksHook) Name() string {
	return checksHookName
}

func (h *checksHook) Prerun() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	group := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)
	if group == nil {
		return nil
	}

	// create and start observers of nomad service checks in alloc
	h.observe(h.alloc, group.NomadServices())

	return nil
}

func (h *checksHook) Update(request *interfaces.RunnerUpdateRequest) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	netlog.Purple("checksHook.Update, id: %s", request.Alloc.ID)

	group := request.Alloc.Job.LookupTaskGroup(request.Alloc.TaskGroup)
	if group == nil {
		return nil
	}

	// get all group and task level services using nomad provider
	services := group.NomadServices()

	// create a set of the updated set of checks
	next := make([]structs.CheckID, 0, len(h.observers))
	for _, service := range services {
		for _, check := range service.Checks {
			next = append(next, structs.MakeCheckID(
				request.Alloc.ID,
				request.Alloc.TaskGroup,
				service.TaskName,
				check.Name,
			))
		}
	}
	netlog.Purple("ch.Update next: %v", next)

	// stop the observers of the checks we are removing
	remove := h.shim.Unwanted(request.Alloc.ID, next)
	for _, id := range remove {
		h.observers[id].stop()
		delete(h.observers, id)
	}

	// purge checks that are no longer part of the allocation
	if err := h.shim.Remove(request.Alloc.ID, remove); err != nil {
		return err
	}

	// remember this new alloc
	h.alloc = request.Alloc

	// ensure we are observing new checks (idempotent)
	h.observe(request.Alloc, services)

	return nil
}

func (h *checksHook) PreKill() {
	h.lock.Lock()
	defer h.lock.Unlock()

	netlog.Yellow("ch.PreKill")

	// terminate the background thing
	netlog.Yellow("ch.PreKill: issue stop")
	h.stop()

	if err := h.shim.Purge(h.allocID); err != nil {
		h.logger.Error("failed to purge check results", "alloc_id", h.allocID, "error", err)
	}
}
