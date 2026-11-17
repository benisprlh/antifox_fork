package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"antifox/agent"
	"antifox/db"
	"antifox/engine/harness"
	"antifox/engine/llm"
	"antifox/intercept"
)

// model_error indicates provider/API failure after transient retries or a broken stream.
// This is external instability, not a completed attempt or definitive failure.
// Retry a few times with backoff rather than permanently blocking the intent.
// Pause/stop/cancellation immediately yields to the corresponding handler.
const (
	modelErrorRetries      = 2               // Additional attempts after model_error.
	modelErrorRetryBackoff = 3 * time.Second // Backoff before each retry.
)

// Engine drives the event-driven exploration loop with real LLM agents
// (docs §4.3/§4.4): on asset/exploration-graph change (debounced) it wakes the
// planner, which reads the route, queries assets, judges goals and emits intents;
// N concurrent work agents claim intents and execute them. There is no
// simulation mode — an LLM provider is required. The planner/worker can be
// (re)installed at runtime (LLM configured from the UI); the loops always run
// but idle until an LLM is set.
type Engine struct {
	m        *Manager
	debounce time.Duration

	bc *Broadcaster // live activity pub/sub (SSE)

	mu      sync.RWMutex
	planner *agent.Planner
	worker  *agent.Worker
	started sync.Map // taskID -> bool, so Run is idempotent per task
	lastAct sync.Map // taskID -> int64 unix, last planner/worker activity (heartbeat)
	paused  sync.Map // taskID -> bool, user-paused (planner + workers idle but loops alive)

	// per-task execution context: each planner.Plan / worker.Execute runs under it,
	// so pausing can CANCEL an in-flight run (not just skip the next one). Recreated
	// on resume since cancelling is one-shot.
	execMu     sync.Mutex
	execCancel map[string]context.CancelFunc
	execCtx    map[string]context.Context

	// per-work (per running intent) cancel, so the planner's kill_work can stop a
	// single worker without pausing the whole task. Keyed by globally-unique intent id.
	workMu     sync.Mutex
	workCancel map[int64]context.CancelFunc

	// steerBox queues planner course-corrections for a running work (keyed by intent
	// id). The worker's PreToolUse hook drains it before its next tool call and hands
	// the message to the model (blocking that call) so it re-plans — no kill needed.
	steerMu  sync.Mutex
	steerBox map[int64][]string

	plannerRound sync.Map // taskID -> int, planner round counter (for UI round separators)

	// Task-level timeout coordination.
	settling     sync.Map // taskID -> bool: settlement started; no new intents dispatched/claimed.
	deadline     sync.Map // taskID -> Unix deadline stamped on first run; 0/absent=unlimited.
	stamped      sync.Map // taskID -> bool: first_run_at stamped once in this process.
	inflight     sync.Map // taskID -> *int64: running planner/worker count for draining.
	coordStarted sync.Map // taskID -> bool: deduplicate deadline coordinators across Run/reload.

	// resolve, if set, returns a task's dedicated planner/worker when it pins a
	// specific LLM profile (nil,nil = use the global active pair). Set once at startup.
	resolve func(t *Task) (*agent.Planner, *agent.Worker)
}

// nextPlannerRound returns the next planner round number for a task (1-based).
func (e *Engine) nextPlannerRound(taskID string) int {
	v, _ := e.plannerRound.LoadOrStore(taskID, 0)
	n := v.(int) + 1
	e.plannerRound.Store(taskID, n)
	return n
}

// Pause stops a task: marks it paused AND cancels any in-flight planner/worker run
// for it (a long worker.Execute would otherwise keep going until it finishes).
func (e *Engine) Pause(taskID string) {
	e.paused.Store(taskID, true)
	e.cancelExec(taskID)
}

// cancelExec cancels a task's current per-task exec context (any in-flight
// planner.Plan / worker.Execute), if present. Shared by Pause and the settle
// sequence's hard-drain backstop.
func (e *Engine) cancelExec(taskID string) {
	e.execMu.Lock()
	if cancel := e.execCancel[taskID]; cancel != nil {
		cancel()
	}
	e.execMu.Unlock()
}

// Resume un-pauses a task and nudges a fresh planning round. The next exec under
// it gets a fresh (uncancelled) context.
func (e *Engine) Resume(t *Task) {
	e.paused.Delete(t.ID)
	t.Notify()
}

// Rerun continues a stopped/finished task; the caller has already reset
// the task to a non-terminal status + cleared its run clock (Manager.ReactivateTask).
// Here we drop the stale deadline, ensure the planner/worker loops are alive, re-arm
// the timeout coordinator, and nudge a planning round. The exploration graph is kept.
func (e *Engine) Rerun(ctx context.Context, t *Task) {
	e.deadline.Delete(t.ID)     // clear stale absolute deadline (re-stamped on next run)
	e.coordStarted.Delete(t.ID) // allow the deadline coordinator to re-arm below
	e.paused.Delete(t.ID)
	e.Run(ctx, t)                      // start loops if not yet running this session; else nudge
	e.startDeadlineCoordinator(ctx, t) // re-arm timeout timer (dedup-safe via coordStarted)
	t.Notify()
}

// execContextFor returns a live per-task context derived from parent, recreating
// it if a prior pause cancelled it.
func (e *Engine) execContextFor(parent context.Context, taskID string) context.Context {
	e.execMu.Lock()
	defer e.execMu.Unlock()
	if e.IsPaused(taskID) {
		// never hand out a live context while paused (guards the claim→Execute race)
		c, cancel := context.WithCancel(parent)
		cancel()
		return c
	}
	if c := e.execCtx[taskID]; c != nil && c.Err() == nil {
		return c
	}
	c, cancel := context.WithCancel(parent)
	e.execCtx[taskID] = c
	e.execCancel[taskID] = cancel
	return c
}

// IsPaused reports whether a task is user-paused.
func (e *Engine) IsPaused(taskID string) bool {
	v, ok := e.paused.Load(taskID)
	return ok && v.(bool)
}

// Started reports whether the engine loops are running for a task.
func (e *Engine) Started(taskID string) bool {
	_, ok := e.started.Load(taskID)
	return ok
}

// LastActivity returns the unix time of the last planner/worker activity for a
// task (0 if none yet).
func (e *Engine) LastActivity(taskID string) int64 {
	if v, ok := e.lastAct.Load(taskID); ok {
		return v.(int64)
	}
	return 0
}

func (e *Engine) touch(taskID string) { e.lastAct.Store(taskID, time.Now().Unix()) }

func NewEngine(m *Manager) *Engine {
	return &Engine{m: m, debounce: 800 * time.Millisecond, bc: NewBroadcaster(),
		execCancel: map[string]context.CancelFunc{}, execCtx: map[string]context.Context{},
		workCancel: map[int64]context.CancelFunc{}, steerBox: map[int64][]string{}}
}

// registerWork records the cancel for the work currently running intentID.
func (e *Engine) registerWork(intentID int64, cancel context.CancelFunc) {
	e.workMu.Lock()
	e.workCancel[intentID] = cancel
	e.workMu.Unlock()
}

// unregisterWork removes and releases the work's cancel (called when Execute returns).
func (e *Engine) unregisterWork(intentID int64) {
	e.workMu.Lock()
	if c := e.workCancel[intentID]; c != nil {
		delete(e.workCancel, intentID)
		c() // release context resources (no-op if already cancelled)
	}
	e.workMu.Unlock()
	e.steerMu.Lock()
	delete(e.steerBox, intentID) // drop any undelivered steering for a finished work
	e.steerMu.Unlock()
}

// SteerWork queues a mid-run course-correction for the work running intentID (the
// planner's steer_work tool). The worker delivers it before its next tool call and
// re-plans — no kill. Errors if no work is currently running that intent.
func (e *Engine) SteerWork(intentID int64, msg string) error {
	if strings.TrimSpace(msg) == "" {
		return fmt.Errorf("steering message must not be empty")
	}
	e.workMu.Lock()
	running := e.workCancel[intentID] != nil
	e.workMu.Unlock()
	if !running {
		return fmt.Errorf("intent %d has no running work; it may have finished or not been claimed", intentID)
	}
	e.steerMu.Lock()
	e.steerBox[intentID] = append(e.steerBox[intentID], msg)
	e.steerMu.Unlock()
	return nil
}

// drainSteer pops the oldest queued steering message for intentID (FIFO), if any.
func (e *Engine) drainSteer(intentID int64) (string, bool) {
	e.steerMu.Lock()
	defer e.steerMu.Unlock()
	q := e.steerBox[intentID]
	if len(q) == 0 {
		return "", false
	}
	msg := q[0]
	if len(q) == 1 {
		delete(e.steerBox, intentID)
	} else {
		e.steerBox[intentID] = q[1:]
	}
	return msg, true
}

// steerHooks wraps the guard's hook runner so the planner can steer a running work:
// before each tool call it drains a queued course-correction (if any) and blocks the
// call, handing the message back to the model — which re-plans its next step instead
// of running the tool. No queued message → the guard behaves exactly as before.
type steerHooks struct {
	inner harness.HookRunner
	drain func() (string, bool)
}

func (h steerHooks) PreToolUse(ctx context.Context, name string, input []byte) (bool, string, []byte) {
	if msg, ok := h.drain(); ok {
		return true, "[Live planner guidance] " + msg +
			"\nThis is immediate guidance for this intent. The current tool call was not executed; adjust your next action accordingly. This guidance takes precedence over conflicting plans.", nil
	}
	if h.inner != nil {
		return h.inner.PreToolUse(ctx, name, input)
	}
	return false, "", nil
}

func (h steerHooks) PostToolUse(ctx context.Context, name string, input, result []byte, isErr bool) {
	if h.inner != nil {
		h.inner.PostToolUse(ctx, name, input, result, isErr)
	}
}

func (h steerHooks) Stop(ctx context.Context, messages []llm.Message) (bool, []string, string) {
	if h.inner != nil {
		return h.inner.Stop(ctx, messages)
	}
	return false, nil, ""
}

// KillWork cancels the in-flight work running intentID (planner's kill_work tool).
// The work's agent-core session honors ctx cancellation and aborts promptly.
func (e *Engine) KillWork(intentID int64) error {
	e.workMu.Lock()
	c := e.workCancel[intentID]
	e.workMu.Unlock()
	if c == nil {
		return fmt.Errorf("intent %d has no running work; it may have finished or not been claimed", intentID)
	}
	c()
	return nil
}

// Broadcaster exposes the engine's live activity pub/sub (used by the SSE handler).
func (e *Engine) Broadcaster() *Broadcaster { return e.bc }

// emitActivity persists one captured step AND fans it out to live subscribers,
// from a single point so storage and the SSE stream never diverge.
func (e *Engine) emitActivity(t *Task, r db.Activity) {
	id, err := e.appendActivity(t, r)
	if err != nil {
		// NO LONGER SILENT: dropping a record breaks command↔result pairing in the
		// trace: a tool_use with a lost tool_result would otherwise appear to run forever.
		log.Printf("[activity] Task %s retries exhausted; dropping record (kind=%s tool=%s tuid=%s): %v",
			t.ID, r.Kind, r.Tool, r.ToolUseID, err)
		e.touch(t.ID)
		return
	}
	r.ID = id
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	e.bc.Publish(t.ID, r)
	e.touch(t.ID)
}

// appendActivity persists one activity row, retrying briefly on write failure.
// Concurrent planner + worker inserts into the same exploration's activity log
// occasionally fail; a couple of quick retries recover most. Crucially, every
// failure is now LOGGED (it used to be swallowed by an `if err == nil`), so the
// underlying DB error is finally visible for diagnosis.
func (e *Engine) appendActivity(t *Task, r db.Activity) (int64, error) {
	var id int64
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if id, err = t.Store.AppendActivity(r); err == nil {
			if attempt > 1 {
				log.Printf("[activity] Task %s write succeeded on retry %d (worker=%s kind=%s tool=%s)",
					t.ID, attempt, r.Worker, r.Kind, r.Tool)
			}
			return id, nil
		}
		log.Printf("[activity] Task %s write failed (attempt %d/3, worker=%s kind=%s tool=%s): %v",
			t.ID, attempt, r.Worker, r.Kind, r.Tool, err)
		time.Sleep(time.Duration(attempt) * 25 * time.Millisecond)
	}
	return 0, err
}

// UseLLM installs (or replaces) the LLM planner + work agent at runtime.
func (e *Engine) UseLLM(p *agent.Planner, w *agent.Worker) {
	e.mu.Lock()
	e.planner, e.worker = p, w
	e.mu.Unlock()
}

func (e *Engine) snapshot() (*agent.Planner, *agent.Worker) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.planner, e.worker
}

// SetAgentResolver installs a per-task planner/worker resolver (wired by the server).
// Called once at startup before any task loop runs, so no lock is needed on reads.
func (e *Engine) SetAgentResolver(fn func(t *Task) (*agent.Planner, *agent.Worker)) {
	e.resolve = fn
}

// snapshotFor returns the planner/worker a task should run on: the task's dedicated
// pair when it pins a specific LLM profile (via the resolver), else the global active
// pair. The resolver returning nil means "no override" (fall back to global).
func (e *Engine) snapshotFor(t *Task) (*agent.Planner, *agent.Worker) {
	if e.resolve != nil {
		if p, w := e.resolve(t); p != nil && w != nil {
			return p, w
		}
	}
	return e.snapshot()
}

// Ready reports whether an LLM provider is configured.
func (e *Engine) Ready() bool {
	p, w := e.snapshot()
	return p != nil && w != nil
}

// Mode reports "llm" when ready, else "idle".
func (e *Engine) Mode() string {
	if e.Ready() {
		return "llm"
	}
	return "idle"
}

// Run starts the planner loop + N worker loops for a task. The loops always run
// but no-op until an LLM is configured (so a task created while idle picks up
// automatically once LLM is set from the UI).
func (e *Engine) Run(ctx context.Context, t *Task) {
	if _, loaded := e.started.LoadOrStore(t.ID, true); loaded {
		t.Notify() // already running — just nudge a planning round
		return
	}
	e.touch(t.ID)
	go e.plannerLoop(ctx, t)
	workers := e.m.Workers()
	for i := 0; i < workers; i++ {
		go e.workerLoop(ctx, t, fmt.Sprintf("work#%d", i+1))
	}
	e.startDeadlineCoordinator(ctx, t) // Deduplicated timer for tasks with timeout > 0.
	t.Notify()                         // kick the first planning round (acted on once LLM is ready)
}

func (e *Engine) plannerLoop(ctx context.Context, t *Task) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.notify:
			// debounce: coalesce a burst of changes into one planning round
			timer := time.NewTimer(e.debounce)
		drain:
			for {
				select {
				case <-t.notify:
				case <-timer.C:
					break drain
				}
			}
			planner, _ := e.snapshotFor(t)
			if planner == nil {
				continue // idle until LLM configured
			}
			if e.IsPaused(t.ID) {
				continue // user-paused: don't plan
			}
			// terminal task (goals all met → done, or failed): the run is over. A
			// resume/nudge — e.g. auto-resume of the active task on restart — must NOT
			// re-plan (it would burn an LLM round and re-confirm a settled result).
			if isTerminalStatus(t.Status) {
				continue
			}
			// During task settlement, discard regular wakeups from worker writes or Resume.
			// The coordinator invokes the final planning round directly through settleTask.
			if e.isSettling(t.ID) {
				continue
			}
			e.stampFirstRun(t) // First actual planning stamps first_run_at and calculates the deadline.
			e.touch(t.ID)
			emit := func(r db.Activity) { e.emitActivity(t, r) }
			ectx := e.clockCtx(e.execContextFor(ctx, t.ID), t, false) // Pause-cancellable context with task deadline.
			log.Printf("[planner] Task %s planning…", t.ID)
			// round marker: each Plan() is one planner round; emit a boundary so the
			// UI can separate rounds in the transcript (kind='round').
			e.emitActivity(t, db.Activity{Worker: "planner", Kind: "round",
				Summary: fmt.Sprintf("Planning round %d", e.nextPlannerRound(t.ID))})
			// which intents' completion triggered this round (may be several — debounce
			// coalesces a burst; empty for non-completion wakes like a new hint).
			doneIntents := t.drainDone()
			e.incInflight(t.ID)
			taskIDInt, _ := strconv.ParseInt(t.ID, 10, 64)
			met, reason, err := planner.Plan(ectx, taskIDInt, e.m.assets, t.Store, t.Goal, doneIntents, emit)
			e.decInflight(t.ID)
			switch {
			case errors.Is(err, harness.ErrModelRefusal):
				e.failRefusedTask(t)
			case err != nil && ectx.Err() == nil:
				log.Printf("[planner] Task %s planning failed: %v", t.ID, err)
			case met:
				log.Printf("[planner] Task %s objective achieved: %s", t.ID, reason)
				// Persist done when all goals are met; the frontend DTO prioritizes terminal states.
				if err := e.m.SetTaskStatus(t.ID, "done"); err != nil {
					log.Printf("[planner] Task %s failed to persist completion: %v", t.ID, err)
				}
				// Completion immediately cancels remaining workers whose intents are no longer needed.
				// The next worker loop refuses new intents; cancelled work follows the completed-task
				// branch below and becomes stopped, not blocked.
				e.cancelExec(t.ID)
			default:
				log.Printf("[planner] Task %s planning complete", t.ID)
			}
			e.touch(t.ID)
		}
	}
}

func (e *Engine) workerLoop(ctx context.Context, t *Task, name string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_, worker := e.snapshotFor(t)
		if worker == nil {
			if sleepCtx(ctx, 1500*time.Millisecond) {
				return
			}
			continue
		}
		if e.IsPaused(t.ID) {
			if sleepCtx(ctx, 1000*time.Millisecond) {
				return
			}
			continue // user-paused: don't claim/execute intents
		}
		if e.isSettling(t.ID) {
			if sleepCtx(ctx, 1000*time.Millisecond) {
				return
			}
			continue // Settlement: claim no new intents; existing work drains through the coordinator.
		}
		if isTerminalStatus(e.m.TaskStatus(t.ID)) {
			if sleepCtx(ctx, 1000*time.Millisecond) {
				return
			}
			continue // Terminal task: do not continue processing leftover frontier intents.
		}
		intent := e.claimNext(t, name)
		if intent == nil {
			if sleepCtx(ctx, 800*time.Millisecond) {
				return
			}
			continue
		}
		log.Printf("[worker %s] Task %s claimed intent #%d", name, t.ID, intent.ID)
		e.stampFirstRun(t) // First actual execution stamps the task clock.
		e.touch(t.ID)
		emit := func(r db.Activity) { e.emitActivity(t, r) }
		ectx := e.clockCtx(e.execContextFor(ctx, t.ID), t, false) // Pause-cancellable context with task deadline.
		// per-work child context so the planner's kill_work can stop just this work.
		workCtx, workCancel := context.WithCancel(ectx)
		e.registerWork(intent.ID, workCancel)
		// wrap the guard hooks so steer_work can inject a mid-run course-correction
		// for THIS intent (drained before the worker's next tool call).
		iid := intent.ID
		taskEmit := func(a db.Activity) {
			nid := iid
			a.NodeID, a.Worker = &nid, name
			emit(a)
		}
		workCtx = intercept.WithTaskContext(workCtx, t.ID, fmt.Sprintf("%s · #%d", name, iid), taskEmit)
		hooks := steerHooks{inner: t.Guard.Hooks(), drain: func() (string, bool) { return e.drainSteer(iid) }}
		wTaskID, _ := strconv.ParseInt(t.ID, 10, 64)
		e.incInflight(t.ID) // Count running work for settlement draining.
		reason, wrote, err := worker.Execute(workCtx, name, wTaskID, e.m.assets, t.Store, intent, hooks, emit, e.m.enrich)
		// Retry model_error only while the work still owns the intent and the task is active,
		// uncancelled, and not settling. Otherwise defer to the appropriate state handler.
		// Avoid consuming other workers' graceful settlement window with retry backoff.
		for attempt := 1; attempt <= modelErrorRetries &&
			reason == harness.ReasonModelError &&
			workCtx.Err() == nil && ectx.Err() == nil && !e.IsPaused(t.ID) && !e.isSettling(t.ID); attempt++ {
			log.Printf("[worker %s] Task %s intent #%d ended with model_error; retrying in %v (%d/%d)",
				name, t.ID, intent.ID, modelErrorRetryBackoff, attempt, modelErrorRetries)
			if sleepCtx(workCtx, modelErrorRetryBackoff) {
				break // Cancelled during backoff; handled by the branch below.
			}
			reason, wrote, err = worker.Execute(workCtx, name, wTaskID, e.m.assets, t.Store, intent, hooks, emit, e.m.enrich)
		}
		e.decInflight(t.ID)
		// CAPTURE kill state BEFORE unregisterWork cancels workCtx. kill = this work's
		// ctx was cancelled (planner kill_work) while the TASK ctx kept running; a
		// pause cancels the task ctx (ectx) instead. Checking workCtx.Err() AFTER
		// unregister would always be true (unregister cancels it) → every completed
		// work would be wrongly marked stopped.
		killed := workCtx.Err() != nil && ectx.Err() == nil
		e.unregisterWork(intent.ID)
		// if a pause cancelled this run mid-flight, return the intent to the frontier
		// so it is re-claimed (and re-run from scratch) on resume — not marked done.
		if ectx.Err() != nil && e.IsPaused(t.ID) {
			_ = t.Store.SetIntentState(intent.ID, "open")
			continue
		}
		// Hard cancellation during task-timeout settlement means exhausted, not blocked.
		// Workers have normally already persisted results in settlement.
		if ectx.Err() != nil && e.isSettling(t.ID) {
			_ = t.Store.SetIntentState(intent.ID, "exhausted")
			log.Printf("[worker %s] Task %s intent #%d settled after timeout (exhausted); wrote %s", name, t.ID, intent.ID, wrote)
			e.touch(t.ID)
			continue
		}
		// Normal task completion cancelled this run; mark it stopped rather than blocked
		// to preserve meaningful intent states for completed tasks.
		if ectx.Err() != nil && isTerminalStatus(e.m.TaskStatus(t.ID)) {
			_ = t.Store.SetIntentState(intent.ID, "stopped")
			log.Printf("[worker %s] Task %s intent #%d cancelled because task completed (stopped)", name, t.ID, intent.ID)
			e.touch(t.ID)
			continue
		}
		// killed by the planner: mark stopped (don't write back results, don't auto-reclaim).
		if killed {
			_ = t.Store.SetIntentState(intent.ID, "stopped")
			log.Printf("[worker %s] Task %s intent #%d terminated (stopped)", name, t.ID, intent.ID)
			e.touch(t.ID)
			t.Notify()
			continue
		}
		if errors.Is(err, harness.ErrModelRefusal) {
			_ = t.Store.SetIntentState(intent.ID, "blocked")
			e.failRefusedTask(t)
			continue
		}
		if err != nil {
			log.Printf("[worker %s] intent %d: %v", name, intent.ID, err)
		}
		// Terminal routing: max_turns -> exhausted (attempted but incomplete), errors -> blocked,
		// normal completion -> done. Exhaustion must not imply permanent coverage.
		state := "done"
		switch {
		case err != nil:
			state = "blocked"
		case reason == harness.ReasonMaxTurns:
			state = "exhausted"
			log.Printf("[worker %s] Intent %d reached turn limit (exhausted); wrote %s", name, intent.ID, wrote)
		case reason == harness.ReasonTimeout:
			state = "exhausted"
			log.Printf("[worker %s] Intent %d timed out (exhausted); settlement wrote %s", name, intent.ID, wrote)
		}
		_ = t.Store.SetIntentState(intent.ID, state)
		log.Printf("[worker %s] Task %s intent #%d ended: %s (wrote %s)", name, t.ID, intent.ID, state, wrote)
		e.touch(t.ID)
		t.NotifyDone(intent.ID) // results changed the graph -> wake the planner (with the just-finished intent id)
	}
}

// A refusal needs user review; another planning round must not silently retry it.
func (e *Engine) failRefusedTask(t *Task) {
	if _, err := e.m.SetTaskStatusGuarded(t.ID, "failed"); err != nil {
		log.Printf("[engine] Task %s failed to persist model refusal: %v", t.ID, err)
	}
	e.cancelExec(t.ID)
	e.touch(t.ID)
	log.Printf("[engine] Task %s stopped: %v", t.ID, harness.ErrModelRefusal)
}

func sleepCtx(ctx context.Context, d time.Duration) (done bool) {
	select {
	case <-ctx.Done():
		return true
	case <-time.After(d):
		return false
	}
}

func (e *Engine) claimNext(t *Task, name string) *db.Node {
	fr, _ := t.Store.Frontier(20)
	for _, in := range fr {
		if ok, _ := t.Store.ClaimIntent(in.ID, name); ok {
			return in
		}
	}
	return nil
}
