package worker

import (
	"context"
	"time"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type TTLWorker struct {
	Plans outbound.PlanRepo
	Jobs  outbound.JobQueue

	Interval time.Duration
	Limit    int
	ClockNow func() time.Time
}

func (w TTLWorker) Run(ctx context.Context) error {
	interval := w.Interval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	limit := w.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	nowFn := w.ClockNow
	if nowFn == nil {
		nowFn = time.Now
	}

	_ = w.tick(ctx, nowFn, limit)

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			_ = w.tick(ctx, nowFn, limit)
		}
	}
}

func (w TTLWorker) tick(ctx context.Context, nowFn func() time.Time, limit int) error {
	if w.Plans == nil || w.Jobs == nil {
		return nil
	}

	nowUnix := nowFn().UTC().Unix()

	items, err := w.Plans.ListLatestAppliedPlansPerIntentWithTTL(ctx, limit)
	if err != nil {
		return err
	}

	for _, it := range items {
		if it.NotAfterUnix == nil {
			continue
		}
		if nowUnix < *it.NotAfterUnix {
			continue
		}

		planID := it.ID

		if existing, ok, err := w.Jobs.FindActiveExpireTTL(ctx, planID); err == nil && ok {
			_ = existing
			continue
		}

		_, _ = w.Jobs.Enqueue(ctx, outbound.JobExpireTTL, map[string]any{
			"planId": planID.String(),
			"actor":  "system:ttl",
		})
	}

	return nil
}
