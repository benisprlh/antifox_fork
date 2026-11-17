package agent

import (
	"context"
	"fmt"

	"antifox/db"
	"antifox/engine/agentcore"
	"antifox/engine/llm"
	"antifox/engine/permission"
	actool "antifox/engine/tool"
	"antifox/engine/transcript"
)

// MainAgent is the thin human-interface orchestrator (docs §4.2 / §7). The human
// chats with it; it observes (read tools), and steers by injecting hints
// (→planner) or direct high-priority intents (→frontier). It does NOT run the
// autonomous intent-generation loop (that is the planner's job).
type MainAgent struct {
	prov        llm.Provider
	model       string
	tx          *transcript.Store // raw LLM conversation persistence (nil = off)
	window      int               // context window in tokens (for compaction)
	maxTurns    int               // max agent turns per run (0 = unlimited)
	proxyAddr   string            // recording proxy for WebFetch (empty = direct)
	proxyCACert string            // recording proxy's CA cert path (HTTPS verify)
	webSearch   WebSearchOpts     // web_search tool backend selection (off by default)
	workDir     string            // shared work dir (surfaced in prompt as artifact-output target)
}

func NewMainAgent(prov llm.Provider, model, workDir string, tx *transcript.Store, window, maxTurns int) *MainAgent {
	return &MainAgent{prov: prov, model: model, workDir: workDir, tx: tx, window: window, maxTurns: maxTurns}
}

// SetProxy points the main agent's WebFetch at the recording proxy plus the CA
// cert it trusts to verify HTTPS through it (empty addr = direct).
func (m *MainAgent) SetProxy(addr, caCert string) { m.proxyAddr, m.proxyCACert = addr, caCert }

// SetWebSearch selects the web_search backend for the main agent (off by default).
func (m *MainAgent) SetWebSearch(o WebSearchOpts) { m.webSearch = o }

// mainAgentDefaultTmpl is the built-in EDITABLE body (section [A]) of the main agent
// prompt, seeded into agent_prompts. Goal is a {{.Goal}} template variable; the
// artifact output rules are code-owned (artifactSpec) and appended after rendering.
const mainAgentDefaultTmpl = `You are the main agent in an authorized penetration testing system and the human operator's interface. You do not explore targets yourself or continuously generate intents; that is the planner's responsibility. Your duties:

1. Observe: Use graph_overview / list_findings / list_facts / list_assets / get_worker_output to answer questions about current progress.
2. Record findings promptly: For every vulnerability verified by a worker or confirmed in your review, **call report_finding immediately, once per finding**. Include vulnclass, severity (critical|high|medium|low; critical means complete system/data compromise, such as RCE, unauthorized arbitrary file reads exposing credentials, or account-takeover authorization bypass), summary including the affected target, and evidence containing actual reproducible requests/commands/responses. Include asset_ids when available. **Provide structured reproduction whenever possible**: entry_point, prerequisites including how test accounts were registered, steps with actual requests/expected outcomes/what they prove, and provenance. Preserve original requests from worker transcripts or execution so a human can reproduce the issue from scratch. **Record confirmed findings without asking permission; listing them in a reply is insufficient.** Only report_finding entries reach Findings, the exploration graph, and reports. Check list_findings first to avoid duplicates. Never report version/CVE matches or speculative exploitability as confirmed. **Exposure and information disclosure also count**: worker-verified unauthorized access to sensitive admin/debug endpoints, Swagger/API docs, exposed configuration/backups/source, or sensitive-data endpoints must also be recorded with actual response evidence. Rate ordinary disclosure low, unauthorized sensitive/admin access medium, and direct takeover or credential exposure high. Do not treat these solely as assets/facts. **When asked to review current findings, or during pause/wrap-up**, record every qualifying verified vulnerability and exposure from the graph, checking list_findings/list_facts to deduplicate. Deliver proven exposures and risks even when the main exploitation objective was not achieved.
3. Steer the system according to the human operator's intent:
   - To change direction, emphasize a vulnerability class, or focus on an area, use add_hint for the planner's next run.
   - To test a specific target immediately, inject a high-priority intent with add_intent (priority 8–10).
4. Reply concisely in English, explaining actions taken and findings recorded.

Current task objective: {{.Goal}}

Do not invent findings. Record and answer only from actual tool results and verified evidence.`

func mainAgentSystem(goal, workDir string) string {
	body := renderSystem("mainagent", mainAgentDefaultTmpl, MainVars{Goal: goal})
	return body + artifactSpec(workDir)
}

// Chat handles one human message and returns the assistant reply. emit, if
// non-nil, receives each execution step (thinking / tool_use / tool_result /
// text / result) so the main-agent session shows its work — exactly like the
// worker/planner sessions — not just the final answer.
func (m *MainAgent) Chat(ctx context.Context, taskID int64, as *db.AssetStore, ts *db.ExplorationStore, goal, message string, emit func(db.Activity), notify func()) (string, error) {
	tsx := NewToolSet(ts, "human")
	if as != nil {
		tsx.SetAssetStore(as, as.Companies())
	}
	tsx.SetTaskID(taskID)
	tsx.SetNotify(notify) // add_hint wakes this task's planner (debounced)
	// Domain tools and the default base tools: Read/Write/Edit/MultiEdit/LS/Glob/Grep/Bash.
	base := append(tsx.MainAgentTools(), actool.DefaultTools()...)
	tools, def, cleanup := AugmentTools(ctx, "mainagent", base)
	defer cleanup()
	system, boundary := deferredSystem(mainAgentSystem(goal, m.workDir), def)
	opts := agentcore.Options{
		Provider:        m.prov,
		SystemPrompt:    system,
		DynamicBoundary: boundary,
		Tools:           tools,
		DeferredTools:   def.Deferred,
		UnlockSet:       def.Unlock,
		PermissionMode:  permission.ModeBypass,
		EnableWebFetch:  true, // Record traffic through the proxy and trust its CA for MITM HTTPS certificates.
		WebFetchProxy:   m.proxyAddr,
		WebFetchCACert:  m.proxyCACert,
		// Optional web search: ddgs needs no key; brave-free needs BraveKey; tavily needs TavilyKey.
		// WebSearchProxy is a separate outbound HTTP/HTTPS/SOCKS5 proxy, independent of MITM recording; empty means direct.
		EnableWebSearch:    m.webSearch.Enabled,
		WebSearchBackend:   m.webSearch.Backend,
		BraveSearchAPIKey:  m.webSearch.BraveKey,
		TavilySearchAPIKey: m.webSearch.TavilyKey,
		WebSearchProxy:     m.webSearch.Proxy,
		BashEnv:            proxyEnv(m.proxyAddr, m.proxyCACert), // Bash commands use the proxy and trust its CA.
		MaxTurns:           m.maxTurns,                           // 0 = unlimited (configurable in agent management)
		Compaction:         compactionConfig(m.window),           // long chats stay within the window
		Todos:              actool.NewTodoStore(),                // Session-local planning todos; discarded on exit.
		// At the step budget, the SDK summarizes progress. Wrap-up prompt/rounds are editable; default is five rounds.
		Settlement: wrapupSettlement("mainagent", nil),
	}
	if m.tx != nil { // persist raw human↔AI conversation; one accumulating file per task
		opts.Transcript = m.tx
		opts.SessionID = fmt.Sprintf("exp%d-main", ts.ID())
	}
	s := agentcore.NewSession(opts)
	defer s.Close()
	// reload the prior conversation from the transcript so the agent has context
	// across turns (each Chat is a fresh session; without this it can't see earlier
	// messages). First turn: no file yet → Resume loads nothing and proceeds.
	if m.tx != nil {
		_ = s.Resume(opts.SessionID)
	}
	// C2: this session is fresh each turn; re-unlock skill-gated MCPs from prior
	// Skill() calls in the reloaded history so revealed tools stay callable.
	seedUnlockFromHistory(s.Messages(), def.UnlockSkill)
	text, _, err := captureRunSession(ctx, s, message, func(r db.Activity) {
		if emit != nil {
			r.Worker = "mainagent"
			emit(r)
		}
	})
	return text, err
}
