package nomad

import (
	"context"
	"time"

	"github.com/armon/go-metrics"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
	limiter "github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"
)

func (srv *Server) CheckRateLimit(limiter *RateLimiter, secretID, op string) error {
	token, err := srv.ResolveSecretToken(secretID)
	if err != nil {
		return err
	}
	return limiter.check(srv.shutdownCtx, token.AccessorID, op)
}

type RateLimiter struct {
	endpoint string
	write    limiter.Store
	read     limiter.Store
	list     limiter.Store
}

func newRateLimiter(endpoint string) *RateLimiter {
	// TODO: needs to take a configuration object so we can set tokens
	// value
	return &RateLimiter{
		endpoint: endpoint,
		write:    newRateLimiterStore(10),
		read:     newRateLimiterStore(10),
		list:     newRateLimiterStore(10),
	}
}

func (r *RateLimiter) check(ctx context.Context, op, key string) error {
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

func (r *RateLimiter) close(ctx context.Context) error {
	// TODO: all the stores have their own goroutine but there's
	// currently no good way to call close on all the RPC endpoints
	// during shutdowns
	var err error
	err = multierror.Append(err, r.write.Close(ctx))
	err = multierror.Append(err, r.read.Close(ctx))
	err = multierror.Append(err, r.list.Close(ctx))
	return err
}

func newRateLimiterStore(tokens uint64) limiter.Store {
	// note: the memorystore implementation never returns an error
	store, _ := memorystore.New(&memorystore.Config{
		Tokens:   tokens,      // Number of tokens allowed per interval.
		Interval: time.Minute, // Interval until tokens reset.
	})
	return store
}
