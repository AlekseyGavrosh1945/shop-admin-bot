// Package poller periodically checks database event sources and pushes
// new events to a notifier. Positions are tracked with per-source
// integer cursors persisted to a state file.
package poller

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"
)

// Event is a single new database row rendered for the user.
type Event struct {
	ID   int64
	Text string
}

// Source is one monitored table/stream.
type Source struct {
	Name  string
	Fetch func(ctx context.Context, since int64, limit int) ([]Event, error)
	MaxID func(ctx context.Context) (int64, error)
}

// Notifier delivers a batch of events (already rendered as lines).
type Notifier func(ctx context.Context, source string, events []Event) error

// Poller polls all sources on a fixed interval.
type Poller struct {
	sources   []Source
	notify    Notifier
	interval  time.Duration
	statePath string
	log       *slog.Logger
	cursors   map[string]int64
	batchSize int
}

// New creates a Poller. Cursors are restored from statePath; sources
// without a stored cursor start from the current MAX(id) so that a first
// run does not replay history.
func New(sources []Source, notify Notifier, interval time.Duration,
	statePath string, log *slog.Logger) *Poller {
	if interval < time.Second {
		interval = time.Second
	}
	return &Poller{
		sources:   sources,
		notify:    notify,
		interval:  interval,
		statePath: statePath,
		log:       log,
		cursors:   map[string]int64{},
		batchSize: 20,
	}
}

// Run blocks until ctx is canceled, polling sources every interval.
func (p *Poller) Run(ctx context.Context) {
	p.restore(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			p.save()
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

// restore loads the state file; sources without a stored cursor are
// initialized to the current MAX(id) without notifying.
func (p *Poller) restore(ctx context.Context) {
	if data, err := os.ReadFile(p.statePath); err == nil {
		var st state
		if err := json.Unmarshal(data, &st); err == nil {
			p.cursors = st.Cursors
		} else {
			p.log.Warn("bad state file, starting fresh", "path", p.statePath, "err", err)
		}
	}
	for _, s := range p.sources {
		if _, ok := p.cursors[s.Name]; ok {
			continue
		}
		maxID, err := s.MaxID(ctx)
		if err != nil {
			p.log.Error("init cursor", "source", s.Name, "err", err)
			maxID = 0
		}
		p.cursors[s.Name] = maxID
	}
	p.save()
}

func (p *Poller) tick(ctx context.Context) {
	for _, s := range p.sources {
		since := p.cursors[s.Name]
		events, err := s.Fetch(ctx, since, p.batchSize)
		if err != nil {
			p.log.Error("poll source", "source", s.Name, "err", err)
			continue
		}
		if len(events) == 0 {
			continue
		}
		if err := p.notify(ctx, s.Name, events); err != nil {
			p.log.Error("notify", "source", s.Name, "err", err)
			continue // keep the cursor so failed events are retried
		}
		p.cursors[s.Name] = events[len(events)-1].ID
		p.save()
	}
}

func (p *Poller) save() {
	data, err := json.Marshal(state{Cursors: p.cursors})
	if err != nil {
		p.log.Error("marshal state", "err", err)
		return
	}
	if err := os.WriteFile(p.statePath, data, 0o600); err != nil {
		p.log.Error("save state", "path", p.statePath, "err", err)
	}
}

// state is the persisted cursors map.
type state struct {
	Cursors map[string]int64 `json:"cursors"`
}
