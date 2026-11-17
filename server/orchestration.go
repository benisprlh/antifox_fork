package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"antifox/agent"
	"antifox/engine/permission"
	actool "antifox/engine/tool"
)

// jsonResult marshals v to a JSON tool result.
func jsonResult(v any) (actool.Result, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return actool.Errorf(err.Error()), nil
	}
	return actool.Text(string(b)), nil
}

// Cross-task orchestration tools live in server because they require Manager stores,
// Engine pause controls, and task creation.
// Read tools reuse per-task logic through a temporary ToolSet targeting the selected store;
// control tools call Manager/Engine directly.
// Like traffic tools, they are seeded into tools and exposed through agent bindings.

// hostTools is the runtime host-tool provider fed to ToolAugment: traffic tools
// (gated by capture) + cross-task orchestration tools + user-defined custom tools.
// The second return is the names of custom tools flagged `deferred` (schema
// withheld, routed via SearchExtraTools/ExecuteExtraTool). Per-agent binding still
// decides who actually sees any of them.
//
//nolint:unused // used as the hostTools provider in wireAgentAugment
func (s *Server) hostTools() ([]actool.CoreTool, map[string][]string) {
	tools := append(s.m.HostTools(), s.orchestrationTools()...)
	tools = append(tools, s.platformTools()...) // Platform management tools for Auto.
	custom, err := s.customTools()
	if err != nil {
		log.Printf("[custom-tool] Loading failed: %v", err)
		return tools, nil
	}
	tools = append(tools, custom...)
	// deferred custom tools → name -> its bound agent keys. ToolAugment turns a
	// name into a deferred entry only for agents it's actually bound to (so we don't
	// advertise a tool the per-agent binding will drop from the callable set).
	deferred := map[string][]string{}
	rows, _ := s.m.pg.ListCustomTools()
	for _, t := range rows {
		if t.Deferred && t.Enabled {
			deferred[t.Key] = t.Agents
		}
	}
	return tools, deferred
}

// orchestrationTools returns the cross-task tool set. Bound per-agent via the
// tools table (default: no binding — opt-in for orchestration agents).
func (s *Server) orchestrationTools() []actool.CoreTool {
	return []actool.CoreTool{
		s.toolListTasks(),
		s.toolListLLMProfiles(),
		s.toolSpawnTask(),
		s.toolPauseTask(),
		s.toolGetTaskGraph(),
		s.toolListTaskFindings(),
		s.toolAddHint(),
		s.toolGetWorkerTrace(),
		s.toolListWorkerTraces(),
		s.toolSearchWorkerTraces(),
	}
}

// --- schema helpers ---

func strParam(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

// parseProfileID reads an LLM profile id from a tool arg that may arrive as a JSON
// number (5) or a numeric string ("5"); returns 0 when absent/unparseable.
func parseProfileID(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var n int64
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		v, _ := strconv.ParseInt(strings.TrimSpace(str), 10, 64)
		return v
	}
	return 0
}

func objSchema(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		req := make([]any, len(required))
		for i, r := range required {
			req[i] = r
		}
		m["required"] = req
	}
	return m
}

func roTool(name, desc string, schema map[string]any, run func(context.Context, json.RawMessage) (actool.Result, error)) actool.CoreTool {
	return actool.Build(actool.Spec{
		Name: name, Description: desc, Schema: schema,
		ReadOnly:   func(json.RawMessage) bool { return true },
		Concurrent: func(json.RawMessage) bool { return true },
		Permissions: func(context.Context, json.RawMessage, permission.Context) permission.Decision {
			return permission.Allowed()
		},
		Run: func(ctx context.Context, in json.RawMessage, _ *actool.ToolContext) (actool.Result, error) {
			return run(ctx, in)
		},
	})
}

func wrTool(name, desc string, schema map[string]any, run func(context.Context, json.RawMessage) (actool.Result, error)) actool.CoreTool {
	return actool.Build(actool.Spec{
		Name: name, Description: desc, Schema: schema,
		Permissions: func(context.Context, json.RawMessage, permission.Context) permission.Decision {
			return permission.Allowed()
		},
		Run: func(ctx context.Context, in json.RawMessage, _ *actool.ToolContext) (actool.Result, error) {
			return run(ctx, in)
		},
	})
}

// delegateToTask resolves the `task_id` in the input, builds a ToolSet bound to
// that task's store, strips task_id, and calls the chosen per-task tool — so the
// cross-task read reuses the exact in-task logic against another task.
func (s *Server) delegateToTask(ctx context.Context, in json.RawMessage, pick func(*agent.ToolSet) actool.CoreTool) (actool.Result, error) {
	var head struct {
		TaskID string `json:"task_id"`
	}
	_ = json.Unmarshal(in, &head)
	if strings.TrimSpace(head.TaskID) == "" {
		return actool.Errorf("task_id is required"), nil
	}
	t, ok := s.m.Task(head.TaskID)
	if !ok {
		return actool.Errorf("task not found: " + head.TaskID), nil
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(in, &m)
	delete(m, "task_id")
	inner, _ := json.Marshal(m)
	tsx := agent.NewToolSet(t.Store, "orchestrator")
	if s.m.Assets() != nil {
		tsx.SetAssetStore(s.m.Assets(), s.m.Assets().Companies())
	}
	tsx.SetNotify(t.Notify) // hint writes wake this task's planner (no-op for read tools)
	return pick(tsx).Call(ctx, inner, nil)
}

// --- tools ---

func (s *Server) toolListTasks() actool.CoreTool {
	return roTool("list_tasks",
		"List tasks with IDs, descriptions, objectives, status, duration, parent, and LLM profile. Use for orchestration and stalled-task monitoring. Duration is creation-to-now for active tasks or creation-to-last-activity for terminal tasks, in seconds. llm_profile names the planner/worker profile; '(Active profile)' follows the global setting.",
		objSchema(map[string]any{}),
		func(context.Context, json.RawMessage) (actool.Result, error) {
			lastAct, _ := s.m.PG().LastActivityAll()
			// id -> name to resolve each task's pinned LLM profile.
			profName := map[int64]string{}
			if profs, err := s.m.pg.ListProfiles(); err == nil {
				for _, p := range profs {
					profName[p.ID] = p.Name
				}
			}
			out := make([]map[string]any, 0)
			for _, t := range s.m.List() {
				status := s.deriveTaskStatus(t)
				end := lastAct[t.ExpID]
				if live := s.engine.LastActivity(t.ID); live > end {
					end = live
				}
				dur := int64(0)
				if status == "running" {
					dur = time.Now().Unix() - t.CreatedAt
				} else if end > t.CreatedAt {
					dur = end - t.CreatedAt
				}
				row := map[string]any{"id": t.ID, "description": t.Description, "goal": t.Goal, "status": status, "run_seconds": dur}
				if t.ParentRef != "" {
					row["parent_ref"] = t.ParentRef
				}
				if t.LLMProfileID == nil {
					row["llm_profile"] = "(Active profile)"
				} else if n, ok := profName[*t.LLMProfileID]; ok {
					row["llm_profile"] = n
				} else {
					row["llm_profile"] = fmt.Sprintf("#%d (deleted)", *t.LLMProfileID)
				}
				out = append(out, row)
			}
			return jsonResult(out)
		})
}

// toolListLLMProfiles lists the available LLM profiles (name/model/active) so an
// orchestration agent can pick one for spawn_task's llm_profile. Never leaks keys.
func (s *Server) toolListLLMProfiles() actool.CoreTool {
	return roTool("list_llm_profiles",
		"List available LLM profiles: ID, name, model, format, and active status, without API keys. Pass an ID to spawn_task.llm_profile_id for a dedicated subtask model.",
		objSchema(map[string]any{}),
		func(context.Context, json.RawMessage) (actool.Result, error) {
			profs, err := s.m.pg.ListProfiles()
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			out := make([]map[string]any, 0, len(profs))
			for _, p := range profs {
				out = append(out, map[string]any{
					"id": p.ID, "name": p.Name, "model": p.Model, "format": p.Format, "is_active": p.IsDefault,
				})
			}
			return jsonResult(map[string]any{"profiles": out})
		})
}

func (s *Server) toolSpawnTask() actool.CoreTool {
	return wrTool("spawn_task",
		"Create a subtask and start its exploration engine, returning task_id. Use for an independent objective/target. Optional parent_ref links it to the current parent task.",
		objSchema(map[string]any{
			"description":     strParam("Task description / short title"),
			"goal":            strParam("Task objective"),
			"parent_ref":      strParam("Optional parent task ID"),
			"llm_profile_id":  map[string]any{"type": "integer", "description": "Optional planner/worker profile ID from list_llm_profiles; omitted inherits the parent, then the global active profile"},
			"timeout_seconds": map[string]any{"type": "integer", "description": "Optional task timeout in seconds; triggers graceful settlement and timeout status. Omitted or 0 means unlimited."},
		}, "description", "goal"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				Description, Goal, ParentRef string
				LLMProfileID                 json.RawMessage `json:"llm_profile_id"`
				TimeoutSeconds               int             `json:"timeout_seconds"`
			}
			_ = json.Unmarshal(in, &a)
			if strings.TrimSpace(a.Description) == "" {
				a.Description = "Untitled task"
			}
			if strings.TrimSpace(a.Goal) == "" {
				return actool.Errorf("goal is required"), nil
			}
			if a.TimeoutSeconds < 0 {
				a.TimeoutSeconds = 0
			}
			// LLM profile resolution: explicit id > inherit parent's pin > active(nil).
			var pin *int64
			if id := parseProfileID(a.LLMProfileID); id > 0 {
				if _, ok := s.loadProfileConfig(id); !ok {
					return actool.Errorf(fmt.Sprintf("LLM profile #%d does not exist or has no API key", id)), nil
				}
				pin = &id
			} else if a.ParentRef != "" {
				if pt, ok := s.m.Task(a.ParentRef); ok {
					pin = pt.LLMProfileID
				}
			}
			t, err := s.m.CreateTask("", a.Description, a.Goal, "", pin, a.TimeoutSeconds)
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			if a.ParentRef != "" {
				t.ParentRef = a.ParentRef
				if id, e := strconv.ParseInt(t.ID, 10, 64); e == nil {
					_ = s.m.PG().SetParentRef(id, a.ParentRef)
				}
			}
			s.seed(t, a.Description+" "+a.Goal) // Seed initial assets for the event-driven loop.
			s.createGoals(s.ctx, t, nil)        // LLM goal decomposition with rule-based fallback.
			s.engine.Run(s.ctx, t)              // Start the exploration engine.
			return actool.Text(fmt.Sprintf("task created: %s", t.ID)), nil
		})
}

func (s *Server) toolPauseTask() actool.CoreTool {
	return wrTool("pause_task", "Pause a task's planner/worker loops.",
		objSchema(map[string]any{"task_id": strParam("Task ID to pause")}, "task_id"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				TaskID string `json:"task_id"`
			}
			_ = json.Unmarshal(in, &a)
			if _, ok := s.m.Task(a.TaskID); !ok {
				return actool.Errorf("task not found: " + a.TaskID), nil
			}
			s.engine.Pause(a.TaskID)
			return actool.Text("task paused: " + a.TaskID), nil
		})
}

func (s *Server) toolGetTaskGraph() actool.CoreTool {
	return roTool("get_task_graph", "Read a task's graph overview: asset counts, frontier, findings, and coverage, as in graph_overview.",
		objSchema(map[string]any{"task_id": strParam("Task ID")}, "task_id"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			return s.delegateToTask(ctx, in, (*agent.ToolSet).GraphOverviewTool)
		})
}

func (s *Server) toolListTaskFindings() actool.CoreTool {
	return roTool("list_task_findings", "Read a task's confirmed findings, including flags/PoCs and id/task_id/intent_id/vulnclass/severity/summary/status.",
		objSchema(map[string]any{"task_id": strParam("Task ID")}, "task_id"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			return s.delegateToTask(ctx, in, (*agent.ToolSet).ListFindingsTool)
		})
}

func (s *Server) toolAddHint() actool.CoreTool {
	return wrTool("add_task_hint", "Inject strategic guidance for the selected task's next planning round.\n"+
		"Prefer batching hints; returned ids preserve order and length, with 0 for failed items. For one hint omit hints and supply top-level text.",
		objSchema(map[string]any{
			"task_id":   strParam("Task ID"),
			"hints":     map[string]any{"type": "array", "description": "Preferred hints array; each item has text/asset_ids.", "items": map[string]any{"type": "object"}},
			"text":      strParam("[Single item] Guidance text"),
			"asset_ids": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Optional asset IDs from the selected task to anchor the hint"},
		}, "task_id"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			return s.delegateToTask(ctx, in, (*agent.ToolSet).AddHintTool)
		})
}

func (s *Server) toolGetWorkerTrace() actool.CoreTool {
	return roTool("get_task_worker_trace",
		"Inspect work in a task: get_task_worker_trace(task_id, intent_id) returns summaries; add step_ids for full details, at most five per call.",
		objSchema(map[string]any{
			"task_id":   strParam("Task ID"),
			"intent_id": map[string]any{"type": "integer", "description": "Intent/work ID within the task"},
			"step_ids":  map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Optional step IDs for full content; at most five"},
		}, "task_id", "intent_id"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			return s.delegateToTask(ctx, in, (*agent.ToolSet).GetWorkerTraceTool)
		})
}

func (s *Server) toolListWorkerTraces() actool.CoreTool {
	return roTool("list_task_worker_traces", "List executed work/intents and step counts in a task to locate useful get_task_worker_trace results.",
		objSchema(map[string]any{"task_id": strParam("Task ID")}, "task_id"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			return s.delegateToTask(ctx, in, (*agent.ToolSet).ListWorkerTracesTool)
		})
}

func (s *Server) toolSearchWorkerTraces() actool.CoreTool {
	return roTool("search_task_worker_traces", "Search all execution traces in a task by keyword, returning matching summaries and intent_id.",
		objSchema(map[string]any{"task_id": strParam("Task ID"), "q": strParam("Search keyword")}, "task_id", "q"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			return s.delegateToTask(ctx, in, (*agent.ToolSet).SearchWorkerTracesTool)
		})
}

// deriveTaskStatus mirrors listTasks' status derivation for the list_tasks tool.
func (s *Server) deriveTaskStatus(t *Task) string {
	switch {
	case isTerminalStatus(t.Status):
		return t.Status
	case t.Paused || s.engine.IsPaused(t.ID):
		return "paused"
	case s.engine.Ready() && s.engine.Started(t.ID):
		return "running"
	}
	return "created"
}

// orchestrationToolSeeds seeds the cross-task tools into the tools table so they
// are bindable per-agent (default: bound to nobody — opt-in for orchestration
// agents). First-insert only, like the traffic seeds.
func (s *Server) seedOrchestrationTools() {
	// Task/platform tools bind by default to Auto, the platform operations assistant.
	// SeedTool handles new rows; seedAutoDefaultBindings migrates existing rows.
	autoAgents, _ := json.Marshal([]string{"auto"})
	for _, t := range s.orchestrationTools() {
		schema, _ := json.Marshal(t.InputSchema())
		_ = s.m.PG().SeedTool(t.Name(), t.Description(), schema, autoAgents)
	}
	for _, t := range s.platformTools() {
		schema, _ := json.Marshal(t.InputSchema())
		_ = s.m.PG().SeedTool(t.Name(), t.Description(), schema, autoAgents)
	}
	s.refreshBuiltinToolSchemas()
	s.refreshReportFindingSchema() // Add structured reproduction to existing tool schemas.
	s.seedAutoDefaultBindings()
	s.seedPlannerDefaultBindings()
	s.seedAutoReportFindingBinding()
	s.seedMainagentReportFindingBinding()
	s.reseedAgentPrompt("mainagent", "prompt_exposure_findings_mainagent_v1", "Record verified exposure/information-disclosure findings during review")
	s.reseedAgentPrompt("worker", "prompt_exposure_findings_worker_v1", "Record verified exposure/information disclosure as findings")
	s.reseedAgentPrompt("pentest", "prompt_exposure_findings_pentest_v1", "Record verified exposure/information disclosure as findings")
	// Autonomy improvements from AntiFox 0.3.4–0.3.6, adapted without a coverage subsystem.
	s.reseedAgentPrompt("planner", "prompt_zerointent_acceptance_v1", "Clarify valid zero-intent rounds and prevent completion before acceptance criteria are met")
	s.reseedAgentPrompt("worker", "prompt_negfact_threshold_v1", "Require inferred confidence for negative conclusions until reasonable methods are exhausted")
	// Record entry point, prerequisites, actual ordered steps, and provenance through report_finding.
	s.reseedAgentPrompt("worker", "prompt_reproduction_recipe_v1", "Record structured reproduction with exact actual requests in report_finding")
	s.reseedAgentPrompt("pentest", "prompt_reproduction_recipe_pentest_v1", "Record structured reproduction in report_finding")
	s.reseedAgentPrompt("mainagent", "prompt_reproduction_recipe_mainagent_v1", "Record structured reproduction in report_finding")
	s.bumpWorkerRunSecondsTo1200() // Increase only untouched worker defaults from 600 to 1200 seconds.
	// Pentest bindings need no migration: BuiltinToolSeeds initializes
	// list_assets/insert_assets/report_finding/list_findings/list_companies
	// together with the pentest agent for new databases.
}

// refreshBuiltinToolSchemas propagates code schema/description changes on the
// orchestration + platform tools into already-seeded rows ONCE per version flag —
// SeedTool only inserts, so new parameters such as spawn_task.llm_profile never
// reaches an old DB otherwise. Preserves each tool's agent binding + enabled flag.
// Bump the flag whenever these tools' schemas/descriptions change in code.
func (s *Server) refreshBuiltinToolSchemas() {
	const flag = "tool_schema_refresh_v4_goal_met_desc"
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	tools := append(s.orchestrationTools(), s.platformTools()...)
	for _, t := range tools {
		schema, _ := json.Marshal(t.InputSchema())
		if err := s.m.pg.RefreshToolDefaults(t.Name(), t.Description(), schema); err != nil {
			log.Printf("[tools] refresh %s schema failed: %v", t.Name(), err)
		}
	}
	// Refresh planner goal_met text too: old wording incorrectly suggested ending a round,
	// which could prematurely mark the entire task complete on an empty planning round.
	for _, sd := range agent.BuiltinToolSeeds() {
		if sd.Key != "goal_met" {
			continue
		}
		schema, _ := json.Marshal(sd.Schema)
		if err := s.m.pg.RefreshToolDefaults(sd.Key, sd.Desc, schema); err != nil {
			log.Printf("[tools] refresh %s desc failed: %v", sd.Key, err)
		}
	}
	_ = s.m.pg.SetSetting(flag, "true")
	log.Printf("[tools] Refreshed orchestration/platform schemas to code defaults once")
}

// seedAutoReportFindingBinding adds "auto" to report_finding's binding ONCE so
// conversation-context agents can call it without requiring an intent_id.
func (s *Server) seedAutoReportFindingBinding() {
	const flag = "auto_report_finding_v1"
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	if err := s.m.pg.AddAgentToToolBinding("auto", []string{"report_finding"}); err != nil {
		log.Printf("[auto] Failed to bind report_finding by default: %v", err)
		return
	}
	_ = s.m.pg.SetSetting(flag, "true")
}

// seedPlannerDefaultBindings adds "planner" to report_finding's binding ONCE
// (guarded by a settings flag), so existing DBs — whose report_finding row was
// seeded as worker-only — also let the planner record findings. Fresh DBs already
// get it via PlannerTools(); this only backfills without overriding a user unbind.
func (s *Server) seedPlannerDefaultBindings() {
	const flag = "planner_report_finding_v1"
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	if err := s.m.pg.AddAgentToToolBinding("planner", []string{"report_finding"}); err != nil {
		log.Printf("[planner] Failed to bind report_finding by default: %v", err)
		return
	}
	_ = s.m.pg.SetSetting(flag, "true")
}

// seedMainagentReportFindingBinding binds report_finding to mainagent
// (mainagent) ONCE. Root-cause fix: the main agent synthesizes/confirms vulns in
// its orchestration conversation but, without this tool, could only READ findings
// (list_findings) — never record them — so its conclusions never became finding
// nodes missing from findings/graph/reports. A migration flag prevents re-adding
// after a deliberate user unbind.
func (s *Server) seedMainagentReportFindingBinding() {
	const flag = "mainagent_report_finding_v1"
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	if err := s.m.pg.AddAgentToToolBinding("mainagent", []string{"report_finding"}); err != nil {
		log.Printf("[mainagent] Failed to bind report_finding by default: %v", err)
		return
	}
	_ = s.m.pg.SetSetting(flag, "true")
	log.Printf("[mainagent] Bound report_finding; main agent can now record findings")
}

// bumpWorkerRunSecondsTo1200 raises the worker single-run wall-clock from the old
// 600s default to 1200s ONCE (guarded), for existing DBs. Only touches the untouched
// 600 default so a user-customized value is preserved. Fresh DBs already seed 1200.
// Runs inside seedOrchestrationTools (before applyLLM builds the worker), so the live
// worker picks up the new SetRunTimeout without a restart-after-migration.
func (s *Server) bumpWorkerRunSecondsTo1200() {
	const flag = "worker_run_seconds_1200_v1"
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	if a, err := s.m.pg.GetAgentByKey("worker"); err == nil && a != nil && a.RunSecs == 600 {
		if err := s.m.pg.SetAgentRunSeconds("worker", 1200); err != nil {
			log.Printf("[worker] Failed to migrate run_seconds to 1200: %v", err)
			return
		}
		log.Printf("[worker] Default run duration increased from 600 to 1200 seconds")
	}
	_ = s.m.pg.SetSetting(flag, "true")
}

// reseedAgentPrompt force-updates ONE agent's DB prompt to the current code default
// ONCE (guarded by a settings flag). DB prompt overrides the code default, so a code
// change alone won't reach existing DBs — this bumps it. Use a FRESH flag whenever the
// code default changes again. Runs before any user edit; won't clobber later user edits
// (the flag is set on first apply).
func (s *Server) reseedAgentPrompt(key, flag, note string) {
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	a, err := s.m.pg.GetAgentByKey(key)
	if err != nil || a == nil {
		return
	}
	tmpl, ok := agent.BuiltinPromptSeeds()[key]
	if !ok {
		return
	}
	// Fresh DBs just seeded this exact template as v1; appending an identical v2 is a
	// cosmetic duplicate in the version history. Skip the write when already current.
	if cur, _ := s.m.pg.CurrentPrompt(a.ID); cur == tmpl {
		_ = s.m.pg.SetSetting(flag, "true")
		return
	}
	if _, err := s.m.pg.SavePrompt(a.ID, tmpl, "System update: "+note, "system"); err != nil {
		log.Printf("[%s] Prompt reseeding failed: %v", key, err)
		return
	}
	_ = s.m.pg.SetSetting(flag, "true")
	log.Printf("[%s] Prompt updated (%s)", key, note)
}

// refreshReportFindingSchema pushes report_finding's CODE schema to the DB tools table
// ONCE (guarded). The runtime offers the DB-stored schema to the agent, and SeedTool is
// first-insert-only — so the newly-added structured `reproduction` param wouldn't reach
// existing DBs' agents without this. Won't clobber user param-default edits after the flag.
func (s *Server) refreshReportFindingSchema() {
	const flag = "tool_schema_refresh_report_finding_repro_v3" // v3: standard HTTP messages, no omitted response proof.
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	for _, sd := range agent.BuiltinToolSeeds() {
		if sd.Key != "report_finding" {
			continue
		}
		schema, _ := json.Marshal(sd.Schema)
		if err := s.m.pg.RefreshToolDefaults(sd.Key, sd.Desc, schema); err != nil {
			log.Printf("[tools] refresh report_finding schema failed: %v", err)
			return
		}
	}
	_ = s.m.pg.SetSetting(flag, "true")
	log.Printf("[tools] Refreshed report_finding schema with structured reproduction")
}

// seedAutoDefaultBindings adds "auto" to the task-op + platform tools' bindings
// ONCE (guarded by a settings flag), so existing DBs whose tool rows were seeded
// before Auto existed still give Auto its default toolset — without re-adding it
// after a user deliberately unbinds.
func (s *Server) seedAutoDefaultBindings() {
	const flag = "auto_default_bindings_v3" // v3: updated asset tools, including insert_assets/add_company_scope.
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	keys := make([]string, 0, len(platformToolKeys)+12)
	for _, t := range s.orchestrationTools() {
		keys = append(keys, t.Name())
	}
	keys = append(keys, platformToolKeys...)
	// Auto needs asset lookup/registration and company scope management.
	keys = append(keys, "insert_assets", "add_company_scope", "list_assets")
	if err := s.m.pg.AddAgentToToolBinding("auto", keys); err != nil {
		log.Printf("[auto] Default binding failed: %v", err)
		return
	}
	_ = s.m.pg.SetSetting(flag, "true")
}
