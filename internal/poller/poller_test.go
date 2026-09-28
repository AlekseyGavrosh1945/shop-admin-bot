package poller

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(os.Stderr, nil)) }

func TestPollerFirstRunStartsFromMaxID(t *testing.T) {
	var notified []Event
	p := New(
		[]Source{{
			Name: "orders",
			Fetch: func(_ context.Context, since int64, _ int) ([]Event, error) {
				return []Event{{ID: since + 1, Text: "x"}}, nil
			},
			MaxID: func(context.Context) (int64, error) { return 100, nil },
		}},
		func(_ context.Context, _ string, events []Event) error {
			notified = append(notified, events...)
			return nil
		},
		time.Hour, filepath.Join(t.TempDir(), "state.json"), testLogger(),
	)
	p.restore(context.Background())

	if p.cursors["orders"] != 100 {
		t.Fatalf("cursor = %d, want 100 (no history replay on first run)", p.cursors["orders"])
	}
	if len(notified) != 0 {
		t.Errorf("first run notified %d events, want 0", len(notified))
	}
}

func TestPollerDeliversNewEventsAndAdvancesCursor(t *testing.T) {
	rows := []Event{{ID: 101, Text: "a"}, {ID: 102, Text: "b"}}
	var batches [][]Event
	p := New(
		[]Source{{
			Name: "orders",
			Fetch: func(_ context.Context, since int64, _ int) ([]Event, error) {
				var out []Event
				for _, e := range rows {
					if e.ID > since {
						out = append(out, e)
					}
				}
				return out, nil
			},
			MaxID: func(context.Context) (int64, error) { return 100, nil },
		}},
		func(_ context.Context, _ string, events []Event) error {
			cp := append([]Event{}, events...)
			batches = append(batches, cp)
			return nil
		},
		time.Hour, filepath.Join(t.TempDir(), "state.json"), testLogger(),
	)
	p.restore(context.Background())
	p.tick(context.Background())

	if len(batches) != 1 || len(batches[0]) != 2 {
		t.Fatalf("got batches %v, want one batch of 2", batches)
	}
	if p.cursors["orders"] != 102 {
		t.Errorf("cursor = %d, want 102", p.cursors["orders"])
	}

	// Nothing new: no notifications, cursor unchanged.
	p.tick(context.Background())
	if len(batches) != 1 {
		t.Errorf("empty tick produced a notification")
	}
}

func TestPollerRetriesAfterNotifyFailure(t *testing.T) {
	rows := []Event{{ID: 101, Text: "a"}}
	fail := true
	p := New(
		[]Source{{
			Name: "orders",
			Fetch: func(_ context.Context, since int64, _ int) ([]Event, error) {
				var out []Event
				for _, e := range rows {
					if e.ID > since {
						out = append(out, e)
					}
				}
				return out, nil
			},
			MaxID: func(context.Context) (int64, error) { return 100, nil },
		}},
		func(context.Context, string, []Event) error {
			if fail {
				return errors.New("telegram down")
			}
			return nil
		},
		time.Hour, filepath.Join(t.TempDir(), "state.json"), testLogger(),
	)
	p.restore(context.Background())

	p.tick(context.Background())
	if p.cursors["orders"] != 100 {
		t.Errorf("cursor advanced despite notify failure: %d", p.cursors["orders"])
	}

	fail = false
	p.tick(context.Background())
	if p.cursors["orders"] != 101 {
		t.Errorf("cursor = %d, want 101 after successful retry", p.cursors["orders"])
	}
}

func TestPollerStateSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	p1 := New(nil, func(context.Context, string, []Event) error { return nil },
		time.Hour, statePath, testLogger())
	p1.cursors = map[string]int64{"orders": 55}
	p1.save()

	p2 := New(nil, func(context.Context, string, []Event) error { return nil },
		time.Hour, statePath, testLogger())
	p2.restore(context.Background())

	if p2.cursors["orders"] != 55 {
		t.Errorf("restored cursor = %d, want 55", p2.cursors["orders"])
	}
}

func TestPollerBatchLimit(t *testing.T) {
	p := New(nil, func(context.Context, string, []Event) error { return nil },
		time.Hour, filepath.Join(t.TempDir(), "state.json"), testLogger())
	if p.batchSize < 1 {
		t.Fatal("batchSize must be positive")
	}
}
