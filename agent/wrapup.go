package agent

import (
	"strings"

	"antifox/engine/harness"
)

// The SDK injects wrap-up/settlement prompts when MaxTurns or the run duration is exhausted,
// allowing the agent to persist already identified results and provide a final summary
// instead of leaving incomplete records.
//
// Each agent's prompt can be overridden through agents.wrapup_prompt; empty uses the built-in.
// Only the prompt body is editable; tool restrictions and settlement policy remain code-owned.

// WrapupOverride, if set, returns the stored wrap-up prompt for an agent key and
// whether a non-empty one exists. Wired by the server to the agents table (like
// PromptOverride for system prompts). nil / empty → the built-in default is used.
var WrapupOverride func(agentKey string) (string, bool)

// WrapupMaxTurnsOverride, if set, returns the admin-configured turn budget for the
// wrap-up phase of an agent and whether a positive one exists. Wired to the agents
// table. nil / ≤0 → the built-in per-agent default (wrapupTurnDefaults) is used.
var WrapupMaxTurnsOverride func(agentKey string) (int, bool)

// Defaults are indexed by agent key. Worker reuses settleWrapUpPrompt in worker.go;
// planner/mainagent have separate defaults, and custom agents use the generic fallback.
var wrapupDefaults = map[string]string{
	"worker":    settleWrapUpPrompt,
	"planner":   plannerWrapUpDefault,
	"mainagent": mainAgentWrapUpDefault,
}

// wrapupTurnDefaults sets settlement's own turn budget; positive DB values can override it.
// Worker gets 4 turns for persistence calls; others get 5. Unknown agents use genericWrapupTurns.
var wrapupTurnDefaults = map[string]int{
	"worker":    4,
	"planner":   5,
	"mainagent": 5,
}

const genericWrapupTurns = 5

const plannerWrapUpDefault = "This planning round is nearly out of steps. Only this round is ending; the system will wake you again when the situation changes, so do not close the entire task. Persist decisions already made without inventing intents to fill the round: zero intents is valid. (1) Batch directions ready for dispatch in one add_intent call. (2) Mark goals proven by facts/findings with prove_goal. (3) Record serial exploitation chains in TodoWrite for later rounds. Then end this round without a summary."

const mainAgentWrapUpDefault = "Your step budget is nearly exhausted and this interaction is ending. Start no new exploration or operations. End with one separate plain-text English sentence summarizing progress, key conclusions, and the recommended next step."

const genericWrapUpDefault = "Your budget is nearly exhausted. Persist completed results that have not been saved, then end with a separate plain-text English sentence summarizing your work and key conclusions; it will be displayed as the run result."

// WrapupDefault returns the built-in default wrap-up prompt for an agent key —
// used by the admin UI as the "restore default" value and empty-field placeholder.
func WrapupDefault(agentKey string) string {
	if d, ok := wrapupDefaults[agentKey]; ok {
		return d
	}
	return genericWrapUpDefault
}

// WrapupTurnsDefault returns the built-in wrap-up turn budget for an agent key —
// used by the admin UI as the "0 = default N" hint.
func WrapupTurnsDefault(agentKey string) int {
	if n, ok := wrapupTurnDefaults[agentKey]; ok {
		return n
	}
	return genericWrapupTurns
}

// resolveWrapup returns the effective wrap-up prompt: the DB override (if set and
// non-empty) over the built-in default.
func resolveWrapup(agentKey string) string {
	if WrapupOverride != nil {
		if t, ok := WrapupOverride(agentKey); ok && strings.TrimSpace(t) != "" {
			return t
		}
	}
	return WrapupDefault(agentKey)
}

// resolveWrapupTurns returns the effective wrap-up turn budget: a positive DB
// override over the built-in per-agent default.
func resolveWrapupTurns(agentKey string) int {
	if WrapupMaxTurnsOverride != nil {
		if v, ok := WrapupMaxTurnsOverride(agentKey); ok && v > 0 {
			return v
		}
	}
	return WrapupTurnsDefault(agentKey)
}

// wrapupSettlement builds the settlement config for an agent's run. Prompt and the
// turn budget are admin-editable per agent; disabled tools are code-owned policy so
// a user can't edit away the "stop probing" guardrail. Resolved fresh each run
// (reads DB live), so edits apply on the next run without a restart.
func wrapupSettlement(agentKey string, disabledTools []string) *harness.Settlement {
	return &harness.Settlement{
		Prompt:        resolveWrapup(agentKey),
		DisabledTools: disabledTools,
		MaxTurns:      resolveWrapupTurns(agentKey),
	}
}

// ---------- Task-level timeout wrap-up prompts ----------
//
// Separate from per-run settlement: per-run means this run's budget ended, whereas task timeout
// means the entire exploration is ending. Planner semantics differ particularly: continue planning
// after per-run settlement, but stop and assess final results on task timeout. Worker/planner only.

// WrapupTaskTimeoutOverride / TurnsOverride provide DB overrides through
// agents.task_timeout_wrapup_prompt / _max_turns (worker/planner only).
var (
	WrapupTaskTimeoutOverride      func(agentKey string) (string, bool)
	WrapupTaskTimeoutTurnsOverride func(agentKey string) (int, bool)
)

var taskTimeoutWrapupDefaults = map[string]string{
	"worker":  workerTaskTimeoutDefault,
	"planner": plannerTaskTimeoutDefault,
}

const workerTaskTimeoutDefault = "The ENTIRE TASK has reached its timeout and is ending, not merely this run. This is your final opportunity: (1) Persist all identified results with insert_assets, record_fact, and report_finding. (2) Start no new commands or probes. (3) End with a separate plain-text English sentence summarizing this intent's key conclusions."

const plannerTaskTimeoutDefault = "The ENTIRE TASK has reached its timeout and is ending, not merely this round. Make a final goal assessment using all current facts and findings. Mark every evidence-supported goal with prove_goal. Create no new intents, because they will not run. End after assessment without a summary."

// TaskTimeoutWrapupDefault returns the built-in prompt for placeholders and reset actions.
func TaskTimeoutWrapupDefault(agentKey string) string {
	return taskTimeoutWrapupDefaults[agentKey] // Empty for unconfigured agents such as mainagent/chat.
}

// resolveTaskTimeoutWrapup prefers a nonempty DB override to the built-in default.
// Empty means this agent has no task-timeout prompt; callers should use per-run text.
func resolveTaskTimeoutWrapup(agentKey string) string {
	if WrapupTaskTimeoutOverride != nil {
		if t, ok := WrapupTaskTimeoutOverride(agentKey); ok && strings.TrimSpace(t) != "" {
			return t
		}
	}
	return TaskTimeoutWrapupDefault(agentKey)
}

func resolveTaskTimeoutTurns(agentKey string) int {
	if WrapupTaskTimeoutTurnsOverride != nil {
		if v, ok := WrapupTaskTimeoutTurnsOverride(agentKey); ok && v > 0 {
			return v
		}
	}
	return resolveWrapupTurns(agentKey) // Default to the per-run settlement turn budget.
}

// wrapupSettlementForTask builds settlement for a worker/planner run that is aware
// of the task deadline. See §5 of the design doc:
//   - clamped=true: timeout means the task deadline, so use task-timeout text;
//     MaxTurns means steps ran out while task time remains, so use per-run text.
//   - clamped=false: both reasons use per-run text, equivalent to wrapupSettlement.
//
// PromptByReason selects using the actual settlement reason, avoiding construction-time mismatch.
func wrapupSettlementForTask(agentKey string, disabledTools []string, clamped bool) *harness.Settlement {
	perRun := resolveWrapup(agentKey)
	st := &harness.Settlement{
		Prompt:        perRun, // Fallback; also used for both reasons when not clamped.
		DisabledTools: disabledTools,
		MaxTurns:      resolveWrapupTurns(agentKey),
	}
	if clamped {
		if tt := resolveTaskTimeoutWrapup(agentKey); tt != "" {
			st.PromptByReason = map[harness.TerminalReason]string{
				harness.ReasonTimeout:  tt,     // Task deadline reached.
				harness.ReasonMaxTurns: perRun, // Steps exhausted while task time remains.
			}
			st.MaxTurns = resolveTaskTimeoutTurns(agentKey)
		}
	}
	return st
}
