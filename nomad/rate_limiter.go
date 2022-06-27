package nomad

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

// CheckRateLimit finds the appropriate limiter for this endpoint and
// operation and returns ErrTooManyRequests if the rate limit has been
// exceeded
func (srv *Server) CheckRateLimit(endpoint, secretID, op string) error {
	srv.rpcRateLimiter.lock.RLock()
	defer srv.rpcRateLimiter.lock.RUnlock()

	limiter, ok := srv.rpcRateLimiter.limiters[endpoint]
	if !ok {
		return fmt.Errorf("no such rate limiter")
	}
	token, err := srv.ResolveSecretToken(secretID)
	if err != nil {
		return err
	}
	return limiter.check(srv.shutdownCtx, token.AccessorID, op)
}

// RateLimiter holds all the rate limiting state
type RateLimiter struct {
	shutdownCtx context.Context
	lock        sync.RWMutex
	limiters    map[string]*endpointLimiter
}

func newRateLimiter(shutdownCtx context.Context, cfg *config.Limits) *RateLimiter {
	rl := &RateLimiter{
		shutdownCtx: shutdownCtx,
		limiters:    map[string]*endpointLimiter{},
	}

	if cfg == nil {
		rl.limiters["Namespace"] = defaultEndpointLimiter("namespace")
		rl.limiters["Job"] = defaultEndpointLimiter("job")
	} else {
		rl.limiters["Namespace"] = newEndpointLimiter("namespace", cfg.Namespace, *cfg)
		rl.limiters["Job"] = newEndpointLimiter("job", cfg.Job, *cfg)
	}

	go func() {
		<-shutdownCtx.Done()
		rl.close()
	}()

	return rl
}

func (rl *RateLimiter) close() {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	// we're already shutting down so provide only a short timeout on
	// this to make sure we don't hang on shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	for _, limiter := range rl.limiters {
		limiter.close(ctx)
	}
}

type endpointLimiter struct {
	endpoint string
	write    limiter.Store
	read     limiter.Store
	list     limiter.Store
}

func defaultEndpointLimiter(endpoint string) *endpointLimiter {
	return &endpointLimiter{
		endpoint: endpoint,
		write:    newRateLimiterStore(math.MaxUint64),
		read:     newRateLimiterStore(math.MaxUint64),
		list:     newRateLimiterStore(math.MaxUint64),
	}

}

func newEndpointLimiter(endpoint string, limits *config.RPCEndpointLimits, defaults config.Limits) *endpointLimiter {

	orElse := func(in *int, defaultVal uint64) uint64 {
		if in == nil || *in < 1 {
			return defaultVal
		}
		return uint64(*in)
	}

	write := orElse(defaults.RPCDefaultWriteRate, math.MaxUint64)
	read := orElse(defaults.RPCDefaultReadRate, math.MaxUint64)
	list := orElse(defaults.RPCDefaultListRate, math.MaxUint64)

	if limits != nil {
		write = orElse(limits.RPCWriteRate, write)
		read = orElse(limits.RPCReadRate, read)
		list = orElse(limits.RPCListRate, list)
	}

	return &endpointLimiter{
		endpoint: endpoint,
		write:    newRateLimiterStore(write),
		read:     newRateLimiterStore(read),
		list:     newRateLimiterStore(list),
	}
}

func (r *endpointLimiter) check(ctx context.Context, op, key string) error {
	var tokens, remaining uint64
	var ok bool
	var err error

	switch op {
	case acl.PolicyWrite:
		tokens, remaining, _, ok, err = r.write.Take(ctx, key)
	case acl.PolicyRead:
		tokens, remaining, _, ok, err = r.read.Take(ctx, key)
	case acl.PolicyList:
		tokens, remaining, _, ok, err = r.list.Take(ctx, key)
	}
	used := tokens - remaining
	metrics.IncrCounterWithLabels(
		[]string{"nomad", "rpc", r.endpoint, op}, 1,
		[]metrics.Label{{Name: "id", Value: key}})
	metrics.AddSampleWithLabels(
		[]string{"nomad", "rpc", r.endpoint, op, "used"}, float32(used),
		[]metrics.Label{{Name: "id", Value: key}})

	if err != nil && err != limiter.ErrStopped {
		return err
	}
	if !ok {
		// if we got ErrStopped we'll also send back
		metrics.IncrCounterWithLabels([]string{"nomad", "rpc", r.endpoint, op, "limited"}, 1,
			[]metrics.Label{{Name: "id", Value: key}})
		return structs.ErrTooManyRequests
	}
	return nil
}

func (r *endpointLimiter) close(ctx context.Context) {
	_ = r.write.Close(ctx)
	_ = r.read.Close(ctx)
	_ = r.list.Close(ctx)
}

func newRateLimiterStore(tokens uint64) limiter.Store {
	// note: the memorystore implementation never returns an error
	store, _ := memorystore.New(&memorystore.Config{
		Tokens:   tokens,      // Number of tokens allowed per interval.
		Interval: time.Minute, // Interval until tokens reset.
	})
	return store
}
