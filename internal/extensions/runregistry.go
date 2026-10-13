package extensions

import (
	"context"
	"sync"
)

// RunRegistry is the list of background runs that are in flight right now, so
// that disabling or uninstalling an integration can stop them.
//
// It mirrors the old host's RunAbortRegistry: one integration id maps to a
// *set* of cancel handles rather than a single slot, because the same
// integration can genuinely have two runs in flight at once (a launcher run
// and a pinned window's rerun), and the external-plugin route has no
// single-slot guard to begin with. Disabling asks the set to cancel; each
// cancellation drops that run's context and the run reports run_killed.
type RunRegistry struct {
	mu   sync.Mutex
	next int
	runs map[string]map[int]context.CancelFunc
}

// NewRunRegistry returns an empty registry. A nil *RunRegistry is also usable:
// every method is a no-op, so an App built without one (a test) keeps working.
func NewRunRegistry() *RunRegistry {
	return &RunRegistry{runs: map[string]map[int]context.CancelFunc{}}
}

// Add registers a run's cancel function under the integration id and returns
// the function that forgets it again. The returned function is idempotent, so
// a run that finishes and a caller that cleans up cannot double-delete the
// handle.
func (r *RunRegistry) Add(id string, cancel context.CancelFunc) func() {
	if r == nil {
		return func() {}
	}
	r.mu.Lock()
	handle := r.next
	r.next++
	if r.runs[id] == nil {
		r.runs[id] = map[int]context.CancelFunc{}
	}
	r.runs[id][handle] = cancel
	r.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			if set := r.runs[id]; set != nil {
				delete(set, handle)
				if len(set) == 0 {
					delete(r.runs, id)
				}
			}
			r.mu.Unlock()
		})
	}
}

// Kill stops every run in flight for the integration and reports how many it
// stopped. The cancels run outside the lock: a run's own completion path takes
// the same lock when its goroutine unregisters, so cancelling inside the lock
// would deadlock a run that finishes at the same moment.
func (r *RunRegistry) Kill(id string) int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	set := r.runs[id]
	cancels := make([]context.CancelFunc, 0, len(set))
	for _, cancel := range set {
		cancels = append(cancels, cancel)
	}
	delete(r.runs, id)
	r.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
	return len(cancels)
}

// InFlight counts the runs a disable or uninstall would stop, for a caller
// that wants to say so before it acts.
func (r *RunRegistry) InFlight(id string) int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.runs[id])
}
