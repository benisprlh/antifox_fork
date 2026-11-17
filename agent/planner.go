package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"antifox/db"
	"antifox/engine/agentcore"
	"antifox/engine/llm"
	"antifox/engine/permission"
	actool "antifox/engine/tool"
	"antifox/engine/transcript"
)

// Planner is the event-driven LLM planner (docs §4.3): each time the asset or
// exploration graph changes (debounced), it reads the exploration route, queries
// assets, judges whether the task goal is met, and emits 0..N exploration intents
// into the frontier. It is the sole intent generator.
type Planner struct {
	prov        llm.Provider
	model       string
	tx          *transcript.Store                      // raw LLM conversation persistence (nil = off)
	window      int                                    // context window in tokens (for compaction)
	maxTurns    int                                    // max agent turns per run (0 = unlimited)
	killWork    func(intentID int64) error             // engine callback to terminate a running work (nil = off)
	steerWork   func(intentID int64, msg string) error // engine callback to steer a running work mid-run (nil = off)
	proxyAddr   string                                 // recording proxy for WebFetch (empty = direct)
	proxyCACert string                                 // recording proxy's CA cert path (HTTPS verify)
	webSearch   WebSearchOpts                          // web_search tool backend selection (off by default)
	workDir     string                                 // shared work dir (surfaced in prompt as artifact-output target)

	// todos keeps ONE plan-scratchpad per task (keyed by exploration id) so the
	// planner's multi-step plan survives across wake-ups — each Plan() is a fresh
	// session, but the shared store lets it record a serial exploit chain once and
	// dispatch it step-by-step over rounds instead of front-loading it in parallel.
	todoMu sync.Mutex
	todos  map[int64]*actool.TodoStore
}

func NewPlanner(prov llm.Provider, model, workDir string, tx *transcript.Store, window, maxTurns int) *Planner {
	return &Planner{prov: prov, model: model, workDir: workDir, tx: tx, window: window, maxTurns: maxTurns, todos: map[int64]*actool.TodoStore{}}
}

// SetProxy points the planner's WebFetch at the recording proxy plus the CA cert
// it trusts to verify HTTPS through it (empty addr = direct).
func (p *Planner) SetProxy(addr, caCert string) { p.proxyAddr, p.proxyCACert = addr, caCert }

// SetWebSearch selects the web_search backend for the planner (off by default).
func (p *Planner) SetWebSearch(o WebSearchOpts) { p.webSearch = o }

// todoFor returns the task's persistent planning todo store, creating it on first
// use. Shared across all of this task's planner wake-ups.
func (p *Planner) todoFor(expID int64) *actool.TodoStore {
	p.todoMu.Lock()
	defer p.todoMu.Unlock()
	s := p.todos[expID]
	if s == nil {
		s = actool.NewTodoStore()
		p.todos[expID] = s
	}
	return s
}

// SetKillWork wires the engine's per-work terminate callback so the planner's
// kill_work tool can stop a single running worker.
func (p *Planner) SetKillWork(fn func(intentID int64) error) { p.killWork = fn }

// SetSteerWork wires the engine's per-work steering callback so the planner's
// steer_work tool can inject a mid-run course-correction into a running worker.
func (p *Planner) SetSteerWork(fn func(intentID int64, msg string) error) { p.steerWork = fn }

// renderPlannerTodos formats the persistent planning todo for injection into the
// wake-up prompt (empty when there are no todos yet — first wake-up).
func renderPlannerTodos(items []actool.Todo) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n[Your planning todos, preserved from the previous wake-up]:\n")
	for _, it := range items {
		mark := map[actool.TodoStatus]string{actool.TodoPending: "☐", actool.TodoInProgress: "▶", actool.TodoCompleted: "✔"}[it.Status]
		if mark == "" {
			mark = "☐"
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", mark, it.Content))
	}
	b.WriteString("Continue from this list. Dispatch only steps whose prerequisites are complete or supported by existing facts. Update TodoWrite and mark fact-satisfied steps completed. Do not redispatch steps already pending or in_progress.")
	return b.String()
}

// renderDoneIntents surfaces the intent(s) whose completion triggered this round
// so the planner first considers fresh yields in the situation summary below before
// deciding whether a new direction opened up. Empty for non-completion wakes.
func renderDoneIntents(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("#%d", id)
	}
	return "\n\n[Intents that just completed and triggered this round]: " + strings.Join(parts, ", ")
}

// renderGraphOverview folds the pre-computed graph_overview snapshot into the
// wake-up prompt so the planner starts each round with the full situation in
// hand — saving the round-trip it would otherwise spend calling the tool. It is
// the exact same JSON graph_overview would return; deeper detail is still one
// tool call away (node_detail / list_facts / …).
func renderGraphOverview(data map[string]any) string {
	b, err := json.Marshal(data)
	if err != nil {
		return "" // fall back to the model calling graph_overview itself
	}
	return "\n\n[Current situation, prefetched from graph_overview; use node_detail/list_facts only for more detail]:\n" + string(b)
}

// plannerDefaultTmpl is the built-in EDITABLE body (section [A]) of the planner prompt,
// seeded into agent_prompts. Goal is a {{.Goal}} variable; artifact output rules
// tail is code-owned (artifactSpec) and appended by plannerSystem after rendering.
const plannerDefaultTmpl = `You are the planner in an authorized penetration testing system. You wake frequently when the graph changes. Read the current situation, assess objectives, and **add exploration intents only when a useful direction is genuinely uncovered**.

Task objective: {{.Goal}}

⚠️ **Producing zero intents is valid and normal when there is a specific reason; it is not a default preference for doing less.** Produce zero intents only when:
- **Already covered**: Every direction is covered by open/running/recent_done intents. Duplicating or rephrasing an existing intent is a serious error; or
- **Waiting on dependencies**: The next step needs output from currently running work. Wait for that work and the next graph-triggered wake-up; dispatching now leaves downstream workers without prerequisites.
Conversely, dispatch an intent when a valuable direction is uncovered and independent of running work, or the objective remains unmet with untested in-scope surfaces. Do not use zero intents as an excuse to stop.

Decision process on every wake-up:
1. **The full situation is attached below, directly from graph_overview; no need to call it again**: task (original title/objective/root node), asset counts, goals/status, open/running/recent_done intents, sites_without_endpoints, confirmed findings count, exploration facts count (distinct from findings), and recent_facts with id/summary/confidence. Recent facts include negative results such as closed ports or non-injectable inputs; avoid redispatching established dead ends. A negative fact marked confidence=inferred is only weak evidence, so one verification intent is appropriate if that direction matters to the objective. **Exploration nodes belong only to this task**, while **the asset graph is global** and asset counts reflect all in-scope assets across tasks. For deeper evidence use list_facts, list_findings, or **node_detail(id)**; list views provide summaries only.
   - open_intents / running_intents / recent_done_intents describe covered or attempted directions. Each intent has **parents** (upstream facts/intents) and **yields** (downstream facts/findings), expressing exploration lineage. Use this to combine evidence into new directions. Recent facts include **from_intent**.
   - sites_without_endpoints / findings indicate directions that may need exploration.
   - Fetch detailed assets only as needed with list_assets (q search, type/company_id/task_id filters, limit/offset pagination, or id/ids), asset_neighbors, and list_findings. The asset graph is global; do not fetch everything by default, and ignore assets unrelated to this task.
2. Assess objectives: graph_overview includes goals and their status. Use prove_goal(goal_id,evidence_id,reason) only when a finding/fact proves an unmet goal. **Proving the last unmet goal automatically completes the task; goal_met is unnecessary.** Complete tasks by proving individual goals.
   ⚠️ **Verify quantitative acceptance criteria before completion.** Before prove_goal or any completion action, compare measured graph state with required counts, privileges, sensitive data, or asset/endpoint coverage. If requirements are unmet, do not mark the goal met; dispatch work to close the gap. Never claim completion because the main portion is done, and never bypass the check with goal_met. For example, one verified flag does not satisfy a goal requiring three flags.
2.5. **Optional, minimal initial reconnaissance**: Bash execution is justified only at the start, when the graph has almost no facts and the initial intent cannot otherwise be made concrete. Perform very few read-only checks, such as one or two homepage/fingerprint curl requests, solely to formulate precise initial intents.
   ⚠️ **You are the planner, not the executor.** Your actions must only formulate intents, never perform work inside the planning phase. The only valid output of this probing is a more precise intent description, not vulnerability discovery, verification, exploitation, or endpoint/directory/parameter enumeration. If tempted to test or confirm something, write an intent for a worker instead.
   **Three mandatory boundaries:**
   - **Once worker facts exist (facts>0 or recent_facts nonempty), do not probe yourself.** Base decisions on facts and either dispatch new intents or end the round. Send promising leads to workers instead of curling them yourself.
   - Even initially, make **at most three probes**, only to clarify initial intents. Do not enumerate endpoints/directories/IDs, follow decoding chains, repeatedly probe one endpoint, or test injections/access control/vulnerabilities. Those are worker tasks. If probing becomes detailed investigation, stop and dispatch the remainder as an intent.
   - Probing helps reasoning but **does not itself create an intent**. Do not probe when existing facts already support a decision.
3. **Decide whether to add exploration directions, with restraint.** Intents are open-ended directions, not fixed categories. Combine known facts, assets, and objectives to identify valuable uncovered next steps, then compare each with open_intents, running_intents, and recent_done_intents:
   - Already covered by an open or running intent: do not duplicate it.
   - Attempted in recent_done_intents, even without results: treat it as blocked and do not repeat unchanged. Reopen only for a materially new mechanism: new facts/assets/parameters or a clearly different approach. Explain the difference in summary. Rephrasing or hoping another attempt succeeds is insufficient. One verification intent for a weak inferred negative result is justified when important to the objective.
   - Generate intents only for directions currently uncovered by any intent.
   - If all known directions are covered, generate no intents and end the round.
   - **Keep routes diverse and avoid premature convergence.** While the objective is unmet, prefer an uncovered, fundamentally different entry surface, asset class, or exploitation chain over synonymous intents on one route. Aim for two or three independent routes, concentrating effort only when a route produces evidence of progress toward the goal. Judge diversity by mechanism, not wording. This does not override zero-intent rules: add a route only when it is distinct and uncovered.
3.5. **Dispatch dependent exploitation chains sequentially.** In a chain such as 1 → 2 → 3, each step needs the previous step's actual output:
   - Do not dispatch all steps in parallel; downstream workers would lack prerequisites and duplicate work.
   - Record the full chain in TodoWrite, then dispatch only the step whose prerequisites are satisfied, usually the first.
   - After its facts arrive, dispatch the next step on the next wake-up and mark satisfied todos completed.
   - Do not split one action into two intents, such as confirming and triggering the same point. Parallel intents are for independent exploration dimensions.
4. Submit the selected new directions in **one** add_intent call using the intents array, at most four highest-value items:
   - summary: One natural-language sentence describing what to do and why. Avoid fixed categories. Make the direction explicit because deduplication mainly compares summaries.
   - asset_ids: IDs of the target assets to test, from list_assets. **Include them whenever available** (zero, one, or multiple). For specific sites/endpoints/parameters/hosts, pass all relevant IDs to link intent coverage to assets. Leave empty only for global reconnaissance with no concrete target asset.
   - parent_ids: Optional upstream node IDs supporting this direction. When combining facts, include all contributing fact IDs; also include parent intent/finding IDs where relevant. Leave empty for a new top-level direction.


Prefer no intent over duplicates or filler. Be concise, restrained, and efficient. Write prose in English.`

func plannerSystem(goal, workDir string) string {
	body := renderSystem("planner", plannerDefaultTmpl, PlannerVars{Goal: goal, Now: time.Now().Format("2006-01-02 15:04:05 MST")})
	return body + artifactSpec(workDir)
}

// Plan runs one planning round. emit, if non-nil, receives the planner's execution
// steps (so users can see how it reads the situation and judges goals — the
// planner is the intent generator and was previously a black box). Returns whether
// the planner judged the goal met.
// doneIntents carries the ids of the intent(s) whose completion triggered this
// round (may be several — the engine debounces a burst; empty for non-completion
// wakes). They are surfaced in the prompt so the planner focuses on the fresh
// yields of what just finished.
func (p *Planner) Plan(ctx context.Context, taskID int64, as *db.AssetStore, ts *db.ExplorationStore, goal string, doneIntents []int64, emit func(db.Activity)) (met bool, reason string, err error) {
	tsx := NewToolSet(ts, "planner")
	if as != nil {
		tsx.SetAssetStore(as, as.Companies())
	}
	tsx.SetTaskID(taskID)
	tsx.killWork = p.killWork   // enable kill_work tool (nil = unavailable)
	tsx.steerWork = p.steerWork // enable steer_work tool (nil = unavailable)
	if origin, _ := ts.OriginFactID(); origin > 0 {
		tsx.SetOwnerNode(origin) // planner-side anchors default to the task root (origin fact)
	}
	// Domain tools and the default base tools: Read/Write/Edit/MultiEdit/LS/Glob/Grep/Bash.
	base := append(tsx.PlannerTools(), actool.DefaultTools()...)
	tools, def, cleanup := AugmentTools(ctx, "planner", base)
	defer cleanup()
	// Place the fresh intents and prefetched graph in the system prompt, not user input:
	// system content survives compaction, stays stable within the round, and benefits from
	// system caching. It follows instructions and precedes deferred tools, which remain last.
	situational := renderDoneIntents(doneIntents) + renderGraphOverview(tsx.graphOverviewData())
	// Task deadline/terminal mode is injected through context; see taskclock.go. On the final
	// round, inject the timeout wrap-up as instructions to assess goals without adding intents.
	tc := taskClockFrom(ctx)
	if tc.Final {
		situational += "\n\n[Final task wrap-up; these instructions override normal planning for this round]: " + resolveTaskTimeoutWrapup("planner")
	}
	system, boundary := deferredSystem(plannerSystem(goal, p.workDir)+situational, def)
	// The planner has no independent wall-clock budget; clamp MaxDuration to the task deadline
	// so timeout uses the task wrap-up prompt, while step exhaustion uses the per-run prompt.
	maxDur, clamped := clampMaxDuration(tc.DeadlineUnix, 0)
	settle := wrapupSettlement("planner", nil)
	if tc.DeadlineUnix > 0 {
		settle = wrapupSettlementForTask("planner", nil, clamped)
	}
	opts := agentcore.Options{
		Provider:        p.prov,
		SystemPrompt:    system,
		DynamicBoundary: boundary,
		Tools:           tools,
		DeferredTools:   def.Deferred,
		UnlockSet:       def.Unlock,
		PermissionMode:  permission.ModeBypass,
		EnableWebFetch:  true, // Record traffic through the proxy and trust its CA for MITM HTTPS certificates.
		WebFetchProxy:   p.proxyAddr,
		WebFetchCACert:  p.proxyCACert,
		// Optional web search: ddgs needs no key; brave-free needs BraveKey; tavily needs TavilyKey.
		// WebSearchProxy is a separate outbound HTTP/HTTPS/SOCKS5 proxy, independent of MITM recording; empty means direct.
		EnableWebSearch:    p.webSearch.Enabled,
		WebSearchBackend:   p.webSearch.Backend,
		BraveSearchAPIKey:  p.webSearch.BraveKey,
		TavilySearchAPIKey: p.webSearch.TavilyKey,
		WebSearchProxy:     p.webSearch.Proxy,
		BashEnv:            proxyEnv(p.proxyAddr, p.proxyCACert), // Bash commands use the proxy and trust its CA.
		MaxTurns:           p.maxTurns,                           // 0 = unlimited (configurable in agent management)
		MaxDuration:        maxDur,                               // Zero means unlimited; otherwise remaining task time.
		Compaction:         compactionConfig(p.window),
		// Share planning todos across wake-ups so sequential chains survive new sessions.
		Todos: p.todoFor(ts.ID()),
		// At the round's step budget, persist decisions through add_intent, prove_goal, and
		// TodoWrite rather than abandoning planning; later graph changes wake the planner again.
		// When clamped to the task deadline, use PromptByReason; see wrapupSettlementForTask.
		Settlement: settle,
	}
	if p.tx != nil { // persist raw LLM conversation; one accumulating file per task's planner
		opts.Transcript = p.tx
		opts.SessionID = fmt.Sprintf("exp%d-planner", ts.ID())
	}
	// The stable system prompt already contains fresh intents and the full graph.
	// User input contains only instructions and the planner's regenerable cross-round todos.
	input := "The graph changed. Plan the next step from the situation in the system prompt and assess objectives. **If no uncovered direction exists, end this round with zero intents, especially while waiting for workers. Never call goal_met merely to end a round: it immediately ends the entire task and is valid only when the objective has actually been achieved with verified results. Otherwise prove individual goals with prove_goal, or simply end the round without a call.**" +
		renderPlannerTodos(opts.Todos.List())
	// Add a hard context deadline with grace when clamped, preventing a stuck round from bypassing soft limits.
	runCtx := ctx
	if maxDur > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, maxDur+settleHardGrace)
		defer cancel()
	}
	_, _, err = captureRun(runCtx, opts, input,
		func(r db.Activity) {
			if emit != nil {
				r.Worker = "planner" // planner activity has no intent_id (it generates them)
				emit(r)
			}
		})
	return tsx.GoalMet, tsx.Reason, err
}
