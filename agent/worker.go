package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"antifox/db"
	"antifox/engine/agentcore"
	"antifox/engine/harness"
	"antifox/engine/llm"
	"antifox/engine/memory"
	"antifox/engine/permission"
	actool "antifox/engine/tool"
	"antifox/engine/transcript"
)

// Worker is an LLM work agent (docs §4.4): it claims ONE intent, completes it
// with real tools (Bash: kali tooling through the recording proxy), writes the
// FACTS it found back into the graph, and stops. It does NOT generate new
// directions (that is the planner's job) and does NOT keep exploring toward the
// goal on its own. Multiple workers run concurrently as goroutines.
// WebSearchOpts is the web-search backend selection the server pushes into each
// agent (planner/worker/main). Enabled=false leaves the web_search tool off.
// Backend is "ddgs" (no key), "brave-free" (BraveKey required), or "tavily"
// (TavilyKey required). It maps directly onto agentcore.Options.
// Proxy is a dedicated egress proxy for the search request (http/https/socks5),
// independent of the traffic-recording MITM proxy — set it when the search endpoint
// is only reachable via a VPN/SOCKS proxy. Empty = direct.
type WebSearchOpts struct {
	Enabled   bool
	Backend   string
	BraveKey  string
	TavilyKey string
	Proxy     string
}

type Worker struct {
	prov        llm.Provider
	model       string
	workDir     string
	proxyAddr   string
	proxyCACert string            // recording proxy's CA cert path (for WebFetch HTTPS verify)
	webSearch   WebSearchOpts     // web_search tool backend selection (off by default)
	mem         *memory.Store     // cross-engagement tradecraft memory (G4)
	tx          *transcript.Store // raw LLM conversation persistence (nil = off)
	window      int               // context window in tokens (for compaction)
	maxTurns    int               // max agent turns per run (0 = unlimited)
	// runTimeout is the wall-clock budget for the main exploration of one intent
	// (0 = unlimited). When it fires, the run is cut and a settlement round is
	// forced so already-identified facts get written back instead of being lost.
	runTimeout time.Duration
	// extraTools are host-provided tools (e.g. traffic query, oast) appended to
	// the worker's graph write-back tools.
	extraTools []actool.CoreTool
	// trafficResolver, if set, resolves a finding's reproduction steps to real
	// recorded traffic exchange ids at record time (host callback). nil = no linkage.
	trafficResolver func(taskID int64, repro json.RawMessage) json.RawMessage
}

// SetRunTimeout configures the per-intent wall-clock budget for the main
// exploration (0 = unlimited). When it fires, the SDK settlement phase still runs
// so facts are never lost to a timeout. Safe to call before Execute.
func (w *Worker) SetRunTimeout(run time.Duration) {
	w.runTimeout = run
}

// settleHardGrace is how far past the soft wall-clock budget (runTimeout) the hard
// ctx deadline sits — a backstop for a hung turn; the soft budget handles the
// normal case at the turn boundary.
const settleHardGrace = 90 * time.Second

// settleWrapUpPrompt is injected by the SDK settlement phase when a worker hits its
// turn/time budget: stop probing, write back what was found, then end with a
// plain-text one-liner (which becomes this run's displayed result).
const settleWrapUpPrompt = "Your budget is nearly exhausted and execution will stop. Do not run further commands or probes. (1) Persist already identified results: new assets with insert_assets, exploration conclusions with record_fact, and confirmed vulnerabilities with report_finding. (2) End with a separate plain-text sentence in English summarizing your work and key conclusions; this sentence is displayed as the run result and is required."

// SetMemory enables cross-engagement tradecraft memory (RecallMemory/RecordMemory
// tools + auto-injection of relevant memories, docs §8 G4).
func (w *Worker) SetMemory(m *memory.Store) { w.mem = m }

func NewWorker(prov llm.Provider, model, workDir string, tx *transcript.Store, window, maxTurns int, extra ...actool.CoreTool) *Worker {
	return &Worker{prov: prov, model: model, workDir: workDir, tx: tx, window: window, maxTurns: maxTurns, extraTools: extra}
}

// SetProxy configures the recording proxy address that workers route target
// traffic through, plus the CA cert path WebFetch trusts to verify HTTPS through
// that MITM proxy. Empty addr disables the hint.
func (w *Worker) SetProxy(addr, caCert string) { w.proxyAddr, w.proxyCACert = addr, caCert }

// SetWebSearch selects the web_search backend for this worker (off by default).
func (w *Worker) SetWebSearch(o WebSearchOpts) { w.webSearch = o }

// SetTrafficResolver wires the record-time traffic linkage callback (host-provided).
func (w *Worker) SetTrafficResolver(fn func(taskID int64, repro json.RawMessage) json.RawMessage) {
	w.trafficResolver = fn
}

// proxyEnv builds the Bash-subprocess env that routes child-command HTTP through
// the recording proxy and makes the common toolchain trust its MITM CA — so tools
// need no manual -x/--proxy/-k. Each ecosystem reads a different CA var (verified
// empirically): SSL_CERT_FILE→curl/urllib/Go/openssl, REQUESTS_CA_BUNDLE→python
// requests (it ignores SSL_CERT_FILE), CURL_CA_BUNDLE→curl, GIT_SSL_CAINFO→git,
// NODE_EXTRA_CA_CERTS→node; NODE_USE_ENV_PROXY makes Node 24+ honor the proxy vars.
// Empty proxyAddr → nil (direct, unchanged env).
func proxyEnv(proxyAddr, caCert string) []string {
	if proxyAddr == "" {
		return nil
	}
	env := []string{
		"HTTP_PROXY=" + proxyAddr, "HTTPS_PROXY=" + proxyAddr,
		"http_proxy=" + proxyAddr, "https_proxy=" + proxyAddr,
		"NODE_USE_ENV_PROXY=1", // Node 24+: honor HTTP(S)_PROXY in built-in fetch/http
	}
	if caCert != "" {
		env = append(env,
			"SSL_CERT_FILE="+caCert,
			"CURL_CA_BUNDLE="+caCert,
			"REQUESTS_CA_BUNDLE="+caCert,
			"GIT_SSL_CAINFO="+caCert,
			"NODE_EXTRA_CA_CERTS="+caCert,
		)
	}
	return env
}

// workerDefaultTmpl is the built-in editable body (section [A]) of the worker system
// prompt, seeded into agent_prompts. The traffic-tool block and artifact output rules
// are NOT here — they are code-owned and appended by workerSystem after rendering
// (sections [B]/[C]) are code-owned, so DB edits cannot remove them.
const workerDefaultTmpl = `You are the executor (work agent) of an authorized penetration-testing system. You receive one intent, a one-sentence exploration direction. Your sole responsibility is to complete that intent, persist discovered facts to the knowledge graph, then stop and return.

Mandatory rules:
1. **Work only on this intent.** Its boundary is strict. A fingerprinting intent allows fingerprinting only, not endpoint enumeration, directory brute force, JS API discovery, or vulnerability testing. The planner assigns those separate intents to other workers.
2. **Do not plan new exploration directions.** Record promising new leads in the graph for the planner instead of pursuing them yourself.
3. **Investigate thoroughly before returning.** One blocked payload, 404, or silent injection does not prove a dead end. Try reasonable encoding, method, parameter, and path variations within this intent. The boundary never changes: do not explore unrelated endpoints or vulnerabilities. Once the direction is exhausted or conclusively blocked, persist results and return. Do not continue merely because the overall task is unfinished or to fill the step budget.
4. **Persist each result immediately in the correct graph.** Unrecorded results may be lost when steps run out. Distinguish the two graphs:
   - **New assets/resources** → insert_assets in the asset graph: endpoints, parameters, technology fingerprints, services, credentials, subdomains. Store status/title/body length/content_type in props.http and technologies as type=tech nodes. Batch multiple assets in the assets array.
   - **Exploration conclusions**, positive or negative → record_fact in the exploration graph with intent_id. Combine related observations into one fact: one-sentence summary plus detail containing stack, status, and response characteristics. Usually one intent produces one fact; do not split every property into a node. Use facts only for genuinely distinct conclusions. Record only new information: review recent_facts in the global situation first and avoid rephrasing existing facts. Confirmation without new observations needs no duplicate fact.
   - **Confirmed vulnerabilities** → report_finding in the exploration graph with PoC and intent_id.
   - **Exposure/disclosure is a finding, not merely an asset.** Actual unauthorized access with response evidence to sensitive management/debug interfaces, Swagger/API docs, configuration, backups/source, or sensitive data is confirmed exposure. Use report_finding, with low for ordinary disclosure, medium for unauthorized sensitive/management access, and high for direct takeover or credentials/keys. Interface existence alone is an asset; unauthorized sensitive content is a finding. Report verified exposures even if the main exploitation objective fails.

Available tools:
- insert_assets: register assets using raw information; code computes keys and parent relationships. For endpoints provide full url+method; code creates domain→site→endpoint and extracts query parameters. Put body/header parameters in params. For fingerprints use type=tech, name, on_url, and props {version,category}. Batch assets and store attributes on their own props; exploration conclusions belong elsewhere.
- record_fact: record conclusions and connect them to intent_id. Combine related observations into one fact; use facts only for distinct conclusions. Record only actual observations with concise evidence (command plus key output) and confidence observed/inferred. Before declaring a negative result, exhaust reasonable methods within the intent. If attempts are incomplete or evidence is uncertain, use inferred so the planner can review it. A careless observed negative can incorrectly close an entire route, especially early in a task.
- report_finding: record confirmed vulnerabilities with PoC and your assigned intent_id only after actually triggering them in this run and obtaining reproducible request/response or command-output evidence. Version/CVE matches, apparently injectable parameters, external databases, changelogs, and code diffs alone are insufficient. Do not substitute CVE searches or patch comparisons for triggering the issue. Record untriggered suspicions with record_fact, confidence=inferred, explaining the suspicion and failed verification for follow-up planning.
  **Always fill structured reproduction.** Include entry_point (method + URL), prerequisites (accounts, roles, prior steps, and how to register test accounts from scratch), ordered steps with exact actual request and response (status, key headers, and essential body), expected conclusion, proves, and provenance of the attack chain/dependencies. Never invent or rewrite request/response evidence. This determines whether a human can reproduce the report accurately.
- list_assets / asset_neighbors query assets; list_facts queries exploration facts; list_findings queries vulnerabilities; node_detail accepts exploration node IDs, never asset IDs.
- Use search_all_worker_traces / list_worker_traces / get_worker_trace only when necessary to recover observations absent from facts. Search across traces with q, list executed work, retrieve summaries by intent_id, then full content with up to five step_ids. This is solely for reusing observations and avoiding duplicate work; your scope remains the assigned intent.

Operate only within authorized scope. When this intent is complete, summarize your work and persisted facts in one English sentence. Be practical, restrained, and focused.`

// workerTrafficBlock is section [B], injected by code only when
// traffic capture is on (proxyAddr set). Not stored, not editable.
func workerTrafficBlock(proxyAddr string) string {
	if proxyAddr == "" {
		return ""
	}
	return "\n\n**Traffic tools**:\n- traffic_search / traffic_get: the recording proxy persists all target traffic under data/traffic/<host>/<request-path>/. Search recorded traffic before requesting the same URL again. traffic_search requires host and defaults to three lightweight index entries (id/method/url/status/resp_len, without response bodies). Increase limit explicitly when needed; use traffic_get(id) for full content."
}

// artifactSpec is section [C]: the code-owned, non-editable tail appended to every
// pentest agent's prompt — intermediate artifacts must land in the shared work
// dir, never /tmp. Guaranteed present regardless of how the DB body is edited.
func artifactSpec(workDir string) string {
	return "\n\n**Artifact output rules**: Write all intermediate scripts, payloads, response bodies, and temporary data to the shared working directory " + workDir + " (the common CWD for all agents; relative paths also resolve here). Do not write to /tmp or other absolute paths."
}

// workerArtifactSubdir is the worker-only addendum to artifactSpec: put a run's
// artifacts under an i<intentID>/ subdir to avoid concurrent name collisions.
const workerArtifactSubdir = "To avoid collisions with other work, put this run's artifacts under i<intent-id>/, for example i123/exploit.py."

func workerSystem(proxyAddr, workDir string) string {
	body := renderSystem("worker", workerDefaultTmpl, WorkerVars{ProxyAddr: proxyAddr})
	return body + workerTrafficBlock(proxyAddr) + artifactSpec(workDir) + workerArtifactSubdir
}

// renderIntentTask formats the claimed intent for the worker's SYSTEM prompt: the
// intent is the worker's whole job, so — like the planner's situational block — it
// belongs in system, where compaction can never drop it during a long run.
func renderIntentTask(intent *db.Node) string {
	return fmt.Sprintf("\n\nAssigned intent (your only task: perform this intent, produce facts, then stop):\n%s\nIntent ID: %d (pass to record_fact / report_finding)", string(intent.Payload), intent.ID)
}

// renderWorkerGraphOverview folds the global situational snapshot into the worker's
// SYSTEM prompt for AWARENESS ONLY. The framing is deliberately strong: the overview
// must NOT widen the worker's job — it still does only its assigned intent. Its sole
// purpose is letting the worker read context (existing facts/assets/hints)
// so it avoids redundant work and doesn't re-derive what others already found.
func renderWorkerGraphOverview(data map[string]any) string {
	b, err := json.Marshal(data)
	if err != nil {
		return "" // fall back silently: the worker just won't have the global context
	}
	return "\n\nGlobal exploration situation (context only, not your task list):\n" +
		"This overview helps you understand progress, reuse conclusions, and avoid duplicating others' work.\n" +
		"It never expands your responsibilities. Perform only your assigned intent. Other open intents, uncovered areas, and possible routes belong to workers scheduled by the planner. Record relevant new leads as facts for the planner; do not pursue them yourself.\n" +
		string(b)
}

// Execute runs one intent. hooks (the per-task Guard) gates every tool call; may
// be nil. emit, if non-nil, receives one ActivityRecord per execution step.
// Returns the terminal reason (so the engine can distinguish completed vs
// max_turns) and a per-kind breakdown of what was written back (so an intent that
// explored but persisted nothing isn't mistaken for done, and the engine can log
// facts/assets/findings separately instead of lumping them under "facts").
func (w *Worker) Execute(ctx context.Context, name string, taskID int64, as *db.AssetStore, ts *db.ExplorationStore, intent *db.Node, hooks harness.HookRunner, emit func(db.Activity), enr EnrichTrigger) (harness.TerminalReason, WriteCounts, error) {
	tsx := NewToolSet(ts, name)
	tsx.SetTaskID(taskID)
	tsx.SetTrafficResolver(w.trafficResolver)
	if as != nil {
		tsx.SetAssetStore(as, as.Companies())
	}
	tsx.SetOwnerNode(intent.ID) // assets this worker discovers anchor to its intent → visible to the task
	tsx.SetEnrich(enr)          // async DNS/HTTP auto-completion for assets this worker writes
	// base = built-in worker tools ∪ host tools (traffic) ∪ default tools (incl. Bash);
	// then augment with the agent's visible skills/MCP. During the SDK settlement
	// phase, Bash is hidden via Settlement.DisabledTools (no local gating needed).
	base := append(tsx.WorkerTools(), w.extraTools...)
	base = append(base, actool.DefaultTools()...)
	tools, def, cleanup := AugmentTools(ctx, "worker", base)
	defer cleanup()

	// Put the intent in the persistent system prompt, resistant to compaction, rather than
	// user input, as the planner does with its situation. Place after instructions, before deferred blocks.
	// Include graphOverviewData for context; put the intent last for emphasis.
	overview := renderWorkerGraphOverview(tsx.graphOverviewData())
	system, boundary := deferredSystem(workerSystem(w.proxyAddr, w.workDir)+overview+renderIntentTask(intent), def)
	// The task deadline injected through context clamps this run's time budget and selects wrap-up text.
	tc := taskClockFrom(ctx)
	maxDur, clamped := clampMaxDuration(tc.DeadlineUnix, w.runTimeout)
	settle := wrapupSettlement("worker", []string{"Bash"})
	if tc.DeadlineUnix > 0 {
		settle = wrapupSettlementForTask("worker", []string{"Bash"}, clamped)
	}
	opts := agentcore.Options{
		Provider:        w.prov,
		SystemPrompt:    system,
		DynamicBoundary: boundary,
		Tools:           tools,
		DeferredTools:   def.Deferred,
		UnlockSet:       def.Unlock,
		PermissionMode:  permission.ModeBypass,
		// WebFetch uses the recording proxy, just like curl. Load the proxy CA so intercepted HTTPS
		// certificates validate normally rather than disabling verification. Empty proxy means direct access.
		EnableWebFetch: true,
		WebFetchProxy:  w.proxyAddr,
		WebFetchCACert: w.proxyCACert,
		// Optional web search: ddgs needs no key; brave-free needs BraveKey; tavily needs TavilyKey.
		// WebSearchProxy is a separate outbound http/https/socks5 proxy, independent of the recording MITM proxy.
		EnableWebSearch:    w.webSearch.Enabled,
		WebSearchBackend:   w.webSearch.Backend,
		BraveSearchAPIKey:  w.webSearch.BraveKey,
		TavilySearchAPIKey: w.webSearch.TavilyKey,
		WebSearchProxy:     w.webSearch.Proxy,
		// Bash HTTP requests default to the recording proxy and trust its CA; no -x/-k needed.
		BashEnv:    proxyEnv(w.proxyAddr, w.proxyCACert),
		WorkingDir: w.workDir,
		MaxTurns:   w.maxTurns, // 0 = unlimited (configurable in agent management)
		// Time budget is checked at turn boundaries without interrupting actions; 0 means unlimited.
		// Clamp to the task deadline so this run enters wrap-up when the task expires (taskclock.go).
		MaxDuration: maxDur,
		// Turn/time exhaustion triggers SDK settlement with Bash hidden, persisting known results.
		// For clamped runs, PromptByReason uses task-timeout text on timeout and per-run text
		// on step exhaustion. Unclamped runs always use per-run text.
		Settlement: settle,
		// large tool output spills to cmd-output/ with a head + pointer (SDK tool.Capture);
		// Full output is preserved on disk; use the SDK default truncation limit of 30,000 characters.
		ToolOutputDir: filepath.Join(w.workDir, "cmd-output"),
		Compaction:    compactionConfig(w.window), // long tool-heavy runs stay within the window
		Todos:         actool.NewTodoStore(),      // Session-local planning todos; discarded on exit.
	}
	if hooks != nil { // typed-nil guard: only set when concrete (avoids harness panic)
		opts.Hooks = hooks
	}
	if w.mem != nil {
		opts.Memory = &agentcore.MemoryOptions{Store: w.mem, AutoInject: true, MaxInject: 3}
	}
	if w.tx != nil { // persist raw LLM conversation; one file per worked intent
		opts.Transcript = w.tx
		opts.SessionID = fmt.Sprintf("exp%d-worker-i%d", ts.ID(), intent.ID)
	}
	intentID := intent.ID
	emitWrap := func(r db.Activity) {
		if emit != nil {
			r.NodeID, r.Worker = &intentID, name
			emit(r)
		}
	}
	// The intent is already in the system prompt; user input only starts execution.
	input := "Execute the intent in the system prompt. Work only on that intent, produce facts, and stop when finished."

	s := agentcore.NewSession(opts)
	defer s.Close() // release the session's background-task manager (temp dir + processes)

	// Budgets + settlement are owned by the SDK (MaxTurns/MaxDuration + Settlement):
	// on hit it runs a wrap-up turn and finishes with ReasonMaxTurns/ReasonTimeout.
	// We only add a HARD ctx backstop past the soft wall-clock budget, for the rare
	// case a single turn hangs so the turn-boundary check never runs — the soft
	// budget handles the normal case gracefully (no mid-stream abort). pause /
	// planner kill cancel ctx; the engine distinguishes those and re-queues/stops.
	// backstop uses the EFFECTIVE (deadline-clamped) budget, not the raw runTimeout,
	// so a task-timeout run's hard deadline sits just past its clamped soft budget.
	runCtx := ctx
	if maxDur > 0 {
		var runCancel context.CancelFunc
		runCtx, runCancel = context.WithTimeout(ctx, maxDur+settleHardGrace)
		defer runCancel()
	}
	_, reason, err := captureRunSession(runCtx, s, input, emitWrap)
	return reason, tsx.Writes(), err
}
