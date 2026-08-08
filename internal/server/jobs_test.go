package server

import (
	"errors"
	"testing"
	"time"
)

func waitForStatus(t *testing.T, m *jobManager, id, want string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		snap, ok := m.snapshot(id)
		if !ok {
			t.Fatalf("job %q missing", id)
		}
		if snap["status"] == want {
			return snap
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %q did not reach status %q (got %q)", id, want, snap["status"])
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestJobManagerLifecycle(t *testing.T) {
	m := newJobManager()
	started := make(chan struct{})
	job := m.start("test_kind", func(setProgress func(any)) (any, error) {
		setProgress(map[string]any{"done": 1, "total": 2})
		close(started)
		time.Sleep(20 * time.Millisecond)
		return map[string]any{"ok": true}, nil
	})

	snap, ok := m.snapshot(job.ID)
	if !ok {
		t.Fatal("snapshot missing for running job")
	}
	if snap["status"] != "running" {
		t.Fatalf("expected running, got %v", snap["status"])
	}
	<-started
	if snap, ok := m.snapshot(job.ID); ok {
		if snap["progress"] == nil {
			t.Fatal("expected progress to be set")
		}
	} else {
		t.Fatal("snapshot missing after progress reported")
	}

	done := waitForStatus(t, m, job.ID, "done")
	if done["result"] == nil {
		t.Fatal("expected result to be set")
	}
	if done["finished_at"] == nil {
		t.Fatal("expected finished_at to be set")
	}

	found := false
	for _, j := range m.list() {
		if j["job_id"] == job.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("finished job not in list")
	}
}

func TestJobManagerError(t *testing.T) {
	m := newJobManager()
	job := m.start("test_err", func(setProgress func(any)) (any, error) {
		return nil, errors.New("boom")
	})
	done := waitForStatus(t, m, job.ID, "error")
	if done["error"] != "boom" {
		t.Fatalf("unexpected error: %v", done["error"])
	}
}

func TestJobManagerUnknownID(t *testing.T) {
	m := newJobManager()
	if _, ok := m.snapshot("nope"); ok {
		t.Fatal("snapshot for unknown id should not exist")
	}
}

func TestJobManagerEviction(t *testing.T) {
	m := newJobManager()
	for i := 0; i < maxJobs+10; i++ {
		m.start("evict", func(setProgress func(any)) (any, error) {
			return nil, nil
		})
	}
	// Wait for every started job to finish (map only shrinks on completion).
	deadline := time.Now().Add(5 * time.Second)
	for {
		all := m.list()
		running := false
		for _, j := range all {
			if j["status"] == "running" {
				running = true
				break
			}
		}
		if !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("jobs did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := len(m.list()); got > maxJobs {
		t.Fatalf("expected at most %d jobs after eviction, got %d", maxJobs, got)
	}
}
