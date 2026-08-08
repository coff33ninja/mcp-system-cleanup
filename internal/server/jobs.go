package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Job is a background work item started by an async tool call. Long-running
// operations (cleaner_run, recycle_empty, cleanup_all, DISM) exceed the MCP
// client's request timeout when run synchronously, so they are dispatched as
// jobs that the client polls with *_poll / *_jobs tools.
type Job struct {
	ID         string    `json:"job_id"`
	Kind       string    `json:"kind"`
	Status     string    `json:"status"` // running | done | error
	Error      string    `json:"error,omitempty"`
	Progress   any       `json:"progress,omitempty"`
	Result     any       `json:"result,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`

	mu sync.Mutex
}

// jobManager owns all in-flight and recently finished jobs.
type jobManager struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

// maxJobs caps the job map; finished jobs are evicted oldest-first beyond this.
const maxJobs = 50

var jobMgr = newJobManager()

func newJobManager() *jobManager {
	return &jobManager{jobs: make(map[string]*Job)}
}

func newJobID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("job-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// start launches fn in a goroutine and returns the job immediately. setProgress
// may be called from fn to expose incremental progress to pollers.
func (m *jobManager) start(kind string, fn func(setProgress func(any)) (any, error)) *Job {
	id := newJobID()
	j := &Job{ID: id, Kind: kind, Status: "running", StartedAt: time.Now()}

	m.mu.Lock()
	m.jobs[id] = j
	m.evictLocked()
	m.mu.Unlock()

	go func() {
		result, err := fn(func(p any) {
			j.mu.Lock()
			j.Progress = p
			j.mu.Unlock()
		})
		j.mu.Lock()
		j.FinishedAt = time.Now()
		if err != nil {
			j.Status = "error"
			j.Error = err.Error()
		} else {
			j.Status = "done"
			j.Result = result
		}
		j.mu.Unlock()

		m.mu.Lock()
		m.evictLocked()
		m.mu.Unlock()
	}()
	return j
}

// evictLocked trims finished jobs once the map grows past maxJobs. Running
// jobs are never evicted. Call with m.mu held.
func (m *jobManager) evictLocked() {
	for len(m.jobs) > maxJobs {
		var oldest *Job
		for _, j := range m.jobs {
			j.mu.Lock()
			done := j.Status == "done" || j.Status == "error"
			fin := j.FinishedAt
			j.mu.Unlock()
			if !done {
				continue
			}
			if oldest == nil || fin.Before(oldest.FinishedAt) {
				oldest = j
			}
		}
		if oldest == nil {
			return
		}
		delete(m.jobs, oldest.ID)
	}
}

func (m *jobManager) get(id string) (*Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	return j, ok
}

// snapshot returns a serializable view of a job.
func (m *jobManager) snapshot(id string) (map[string]any, bool) {
	j, ok := m.get(id)
	if !ok {
		return nil, false
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return map[string]any{
		"job_id":      j.ID,
		"kind":        j.Kind,
		"status":      j.Status,
		"error":       j.Error,
		"progress":    j.Progress,
		"result":      j.Result,
		"started_at":  j.StartedAt,
		"finished_at": j.FinishedAt,
	}, true
}

// list returns a compact view of recent jobs (no progress or results),
// newest first.
func (m *jobManager) list() []map[string]any {
	m.mu.Lock()
	all := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		all = append(all, j)
	}
	m.mu.Unlock()

	sort.Slice(all, func(i, j int) bool { return all[i].StartedAt.After(all[j].StartedAt) })

	out := make([]map[string]any, 0, len(all))
	for _, j := range all {
		j.mu.Lock()
		out = append(out, map[string]any{
			"job_id":      j.ID,
			"kind":        j.Kind,
			"status":      j.Status,
			"error":       j.Error,
			"started_at":  j.StartedAt,
			"finished_at": j.FinishedAt,
		})
		j.mu.Unlock()
	}
	return out
}
