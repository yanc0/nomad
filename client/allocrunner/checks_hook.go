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
type observers map[checks.ID]*observer

// An observer is used to execute a particular check on its interval and update the
// check store with those results.
type observer struct {
	ctx     context.Context
	shim    checkstore.Shim
	checker checks.Checker

	allocID string
	check   *structs.ServiceCheck
	qc      *checks.QueryContext
}

func (o *observer) start() {
	timer, cancel := helper.NewSafeTimer(0)
	defer cancel()

	netlog.Cyan("observer started for check: %s", o.check.Name)

	for {
		select {
		case <-o.ctx.Done():
			netlog.Cyan("observer exit, check: %s", o.check.Name)
			return
		case <-timer.C:

			// execute the check
			query := checks.GetQuery(o.check)
			result := o.checker.Do(o.qc, query)

			netlog.Cyan("observer result: %s ...", result)
			netlog.Cyan("%s", result.Output)

			// and put the results into the store
			_ = o.shim.Set(o.allocID, result)

			timer.Reset(o.check.Interval)
		}
	}
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
		shim:    shim,
		network: network,
		checker: checks.New(logger, alloc),
	}
	h.initialize(alloc)
	return h
}

// initialize the dynamic fields of checksHook, which is to say setup all the
// observers and query context things associated with alloc.
//
// Should be called during initial setup and on allocation updates.
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

	var ports structs.AllocatedPorts
	var networks structs.Networks
	if alloc.AllocatedResources != nil {
		ports = alloc.AllocatedResources.Shared.Ports
		networks = alloc.AllocatedResources.Shared.Networks
	}

	netlog.Purple("ports:", ports)
	netlog.Purple("networks:", networks)

	setup := func(name string, services []*structs.Service) {
		for _, service := range services {
			if service.Provider != structs.ServiceProviderNomad {
				continue
			}
			for _, check := range service.Checks {
				id := checks.MakeID(alloc.ID, alloc.TaskGroup, name, check.Name)
				h.observers[id] = &observer{
					ctx:     h.ctx,
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
			}
		}
	}

	// init for nomad group services
	setup("group", tg.Services)

	// init for nomad task services
	for _, task := range tg.Tasks {
		setup(task.Name, task.Services)
	}
}

func (h *checksHook) Name() string {
	return checksHookName
}

func (h *checksHook) Prerun() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	now := time.Now().UTC().Unix()
	netlog.Yellow("ch.PreRun, now: %v", now)

	// insert a pending result into state store for each check
	for _, obs := range h.observers {
		result := checks.Stub(obs.qc.ID, checks.GetKind(obs.check), now)
		if err := h.shim.Set(h.allocID, result); err != nil {
			return err
		}
	}

	// start the observers
	for _, obs := range h.observers {
		go obs.start()
	}

	return nil
}

func (h *checksHook) Update(request *interfaces.RunnerUpdateRequest) error {
	netlog.Yellow("checksHook.Update, id: %s", request.Alloc.ID)

	netlog.Yellow("ch.Update: issue stop")

	// todo: need to reconcile check store, may be checks to remove

	return nil
}

func (h *checksHook) PreKill() {
	netlog.Yellow("ch.PreKill")

	// terminate the background thing
	netlog.Yellow("ch.PreKill: issue stop")
	h.stop()

	if err := h.shim.Purge(h.allocID); err != nil {
		h.logger.Error("failed to purge check results", "alloc_id", h.allocID, "error", err)
	}
}
