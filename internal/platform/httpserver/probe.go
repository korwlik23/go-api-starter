package httpserver

import (
	"context"
	"errors"
	"time"
)

var errProbePanicked = errors.New("readiness probe panicked")

type Probe struct {
	Name     string
	Critical bool
	Check    func(context.Context) error
}

type Readiness struct {
	Timeout time.Duration
	Probes  []Probe
}

func (readiness Readiness) Ready(parent context.Context) bool {
	if readiness.Timeout <= 0 {
		return false
	}

	ctx, cancel := context.WithTimeout(parent, readiness.Timeout)
	defer cancel()

	for _, probe := range readiness.Probes {
		if !probe.Critical {
			continue
		}
		if probe.Check == nil || ctx.Err() != nil {
			return false
		}
		if err := runProbe(ctx, probe.Check); err != nil {
			return false
		}
	}

	return ctx.Err() == nil
}

func runProbe(ctx context.Context, check func(context.Context) error) (err error) {
	defer func() {
		if recover() != nil {
			err = errProbePanicked
		}
	}()
	return check(ctx)
}
