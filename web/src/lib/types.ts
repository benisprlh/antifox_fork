// AntiFox domain model — types used across the UI.
// Types derived from the functional specification's data-shape section.

export type TaskStatus = "created" | "running" | "paused" | "done" | "failed" | "timeout";
export type EngineMode = "exploring" | "paused" | "stalled" | "idle";

export interface Task {
  id: string;
  name?: string; // Short task name; empty falls back to ID and description.
  description: string;
  goal: string;
  status: TaskStatus;
  created_at: string;
  created_unix?: number; // created_at as unix seconds (run-duration calc)
  completed_at?: string; // RFC3339 finish time (done/failed); "" if unfinished
  completed_unix?: number; // completed_at as unix seconds (0/undef if unfinished)
  last_activity_unix?: number; // unix seconds of the last activity (0/undef if none)
  paused?: boolean;
  active?: boolean;
  in_flight?: number;
  last_activity?: string;
  stalled?: boolean;
  goals_total?: number;
  goals_met?: number;
  engine_mode?: EngineMode;
  tokens?: TokenTotal; // whole-task token consumption
  llm_profile_id?: number; // LLM profile used; absent = default profile
  timeout_seconds?: number; // Task timeout seconds; 0/absent=unlimited, used by edit forms.
  egress?: string; // Empty=global; direct=bypass proxy; proxy=global proxy.
}

// ---- Asset graph (global, shared across tasks) ----
export type AssetType =
  | "company"
  | "domain"
  | "ip"
  | "port"
  | "service"
  | "site"
  | "endpoint"
  | "parameter"
  | "tech"
  | "credential"
  | "data";

export type NodeState = "observed" | "confirmed" | "tombstoned";

export interface AssetNode {
  id: string;
  type: AssetType;
  name: string;
  key: string; // nkey
  value?: string;
  company_id?: string; // Owning company asset ID; empty=unassigned.
  state: NodeState;
  confidence: number; // 0..1
  attrs?: Record<string, unknown>;
  first_seen: string;
  last_seen: string;
}

export type AssetRel =
  | "owns"
  | "resolves"
  | "exposes"
  | "runs"
  | "serves"
  | "has_endpoint"
  | "has_param"
  | "fingerprinted"
  | "authenticates_as"
  | "reachable"
  | "has_subdomain";

export interface Edge {
  src: string;
  dst: string;
  rel: AssetRel | ExploreRel;
}

// Task asset view — server-side enriched, paginated.
export interface TaskAssetRef {
  id: string;
  name?: string;
  key: string;
  attrs?: Record<string, unknown>;
}

export interface TaskAssetItem extends AssetNode {
  techs?: TaskAssetRef[];
  auth?: TaskAssetRef[];
  params?: TaskAssetRef[];
}

export interface TaskAssetView {
  counts: Record<string, number>;
  total: number;
  items: TaskAssetItem[];
}

// ---- New unified asset model (new backend) ----
export type NewAssetType = "root_domain" | "ip" | "subdomain" | "app" | "service" | "endpoint";

export interface Asset {
  id: number;
  type: NewAssetType;
  company_id?: number;
  task_ids: number[];
  domain?: string;
  root_domain?: string;
  ip?: string;
  c_segment?: string;
  port?: number;
  icp?: string;
  bound_domains?: string[];
  open_ports?: { port: number; service?: string }[];
  record_type?: string;
  record_value?: string[] | string;
  bundle_id?: string;
  app_name?: string;
  category?: string;
  app_description?: string;
  app_icp?: string;
  url?: string;
  service_type?: string;
  service_name?: string;
  favicon_mmh3?: string;
  status_code?: number;
  content_length?: number;
  page_title?: string;
  technologies?: string[];
  auth?: Record<string, unknown>[];
  method?: string;
  params?: Record<string, unknown>[];
  extra?: Record<string, unknown>;
  last_seen: string;
}

// One company-scope rule, the authoritative ownership source.
export interface ScopeRow {
  id: number;
  company_id: number;
  kind: "domain" | "ip" | "cidr";
  domain?: string;  // Present for kind=domain.
  net?: string;     // Present for kind=ip/cidr.
  raw: string;      // Original input for display/editing.
  reason?: string;
}

// Company asset node with logo, counts, and scope rules.
export interface Company {
  id: number;
  name: string;
  logo?: string; // Remote logo URL; empty falls back to name initials.
  asset_count: number;
  scope: ScopeRow[];
}

// ---- Exploration graph (per task) ----
export type ExploreKind = "task" | "begin" | "goal" | "intent" | "fact" | "finding" | "hint";
export type GoalState = "open" | "met" | "abandoned";
export type IntentState = "open" | "running" | "done" | "blocked" | "exhausted" | "stopped";
export type FindingState = "confirmed" | "dismissed";
export type HintState = "active" | "consumed";
export type ExploreRel = "spawns" | "derived_from" | "yields" | "proves";

export interface TaskNode {
  id: string;
  type: ExploreKind;
  payload?: string;
  priority: number; // 0..10
  state: string; // GoalState | IntentState | FindingState | HintState
  origin: string;
  ts: string;
}

// ---- Findings ----
export type Severity = "critical" | "high" | "medium" | "low";

export interface Finding {
  id: string;
  node_id?: string;
  vulnclass: string;
  severity: Severity;
  summary: string;
  evidence: string;
  intent_id?: string;
  param_id?: string;
  task_id?: string;
  task_description?: string;
  ts: string;
  // enrichment summary (merged in the list)
  enr_name?: string;
  enr_target?: string;
  status?: FindingStatus;
  enriched?: boolean;
  cvss_score?: number;
  evidence_count?: number;
  traffic_count?: number;
}

export type FindingStatus = "open" | "confirmed" | "fixed" | "false_positive" | "ignored";

// Full structured AI/manual enrichment for a finding.
export interface Enrichment {
  node_id: string;
  name: string;
  target: string;
  module: string;
  severity: string;
  status: FindingStatus;
  cvss_score: number;
  cvss_vector: string;
  cvss_metrics: Record<string, string>;
  description: string[];
  test_process: { type: string; content: string }[];
  risk_analysis: string[];
  remediation: string[];
  source: string;
  updated_at?: string;
}

// Agent-recorded reproduction supports explicit steps and per-step evidence.
export interface ReproStep {
  request: string;
  response: string;
  expected: string;
  proves: string;
  // Recorded traffic ID for packet/screenshot navigation.
  exchange_id?: string;
}
export interface Reproduction {
  entry_point?: string;
  prerequisites?: string[];
  steps?: ReproStep[];
  provenance?: string;
}

export interface FindingDetailResp {
  finding: Finding;
  enrichment: Enrichment | null;
  reproduction?: Reproduction | null;
}

// Evidence screenshots shared by findings/reports/Markdown; step_index=0 means overview.
export interface FindingEvidence {
  id: number;
  node_id: number;
  filename: string;
  caption: string;
  step_index: number;
  sort_order: number;
  created_at: string;
}
export interface FindingEvidenceItem {
  evidence: FindingEvidence;
  url: string;
}

// Asynchronous AI-analysis state from POST enrich / GET enrich/status.
export interface EnrichJob {
  status: "analyzing" | "done" | "failed" | "idle";
  error?: string;
  started_at?: string;
  enrichment?: Enrichment;
}

// ---- Activity / sessions ----
export type ActivityKind =
  | "tool_use"
  | "tool_result"
  | "text"
  | "thinking"
  | "result"
  | "user"
  | "intent"            // LLM-generated exploration objective leading a worker session (UI-synthesized)
  | "round"             // planner round boundary marker (engine-emitted)
  | "usage"             // live cumulative token usage (per model turn); not rendered
  | "intercept_request"; // user-approval request from the intercept layer

export interface Activity {
  seq: number;
  intent_id?: string;
  worker: string; // session owner: planner | mainagent | work#1 ...
  ts: string;
  kind: ActivityKind;
  tool?: string;
  tool_use_id?: string;
  is_error?: boolean;
  summary: string;
  detail?: string;
  // token usage (present only on kind='result')
  input_tokens?: number;
  output_tokens?: number;
  cache_read_tokens?: number;
  cache_write_tokens?: number;
}

// ---- Custom-agent scheduled/event triggers ----
export interface AgentTrigger {
  id: number;
  agent_key: string;
  enabled: boolean;
  interval_sec: number; // Run every N seconds; 0 disables the schedule.
  on_finding: boolean; // Trigger when any task records a finding.
  on_goal_met: boolean; // Trigger when any task achieves its objective.
  on_task_timeout: boolean; // Trigger when any task times out.
  interval_message: string; // Independent user messages for each trigger condition.
  finding_message: string;
  goal_message: string;
  task_timeout_message: string;
  last_fire?: string;
}

// ---- Conversations (chat page) ----
export interface Conversation {
  id: number;
  agent_key: string;
  title: string;
  llm_profile_id?: number;
  created_at: string;
  updated_at: string;
}

// ---- Backend logs (/logs page) ----
export interface LogLine {
  seq: number;
  db_id?: number; // server_logs.id; present for DB-persisted lines
  ts: string;
  level: "info" | "warn" | "error";
  tag: string;
  text: string;
}

export type SessionRole = "mainagent" | "planner" | "worker";
export type SessionStatus = "running" | "done" | "blocked" | "exhausted" | "pending" | "stopped";

// Daily token aggregate bucket (GET /api/tokens/daily).
export interface DailyTokenBucket {
  date: string; // "YYYY-MM-DD"
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
}

// Per-worker token usage (GET /api/exploration/tokens).
export interface TokenUsage {
  worker: string;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
}

// Whole-task (all agents) token aggregate.
export interface TokenTotal {
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
}

export interface Session {
  id: string;
  role: SessionRole;
  title: string;
  status: SessionStatus;
  live: boolean;
  last_activity: string;
  intent_id?: string;
}

// ---- Security ----
export interface AuditEntry {
  ts: string;
  tool: string;
  action: "allow" | "block";
  reason?: string;
  command?: string;
}

export interface Audit {
  entries?: AuditEntry[];
  attributions?: Record<string, number>;
}

// ---- Traffic ----
export interface TrafficExchange {
  id: string;
  ts: string;
  host: string;
  method: string;
  url: string;
  status: number;
  content_type: string;
  resp_len: number;
  // Body-search snippet wraps matches in \x01/\x02; present only during body filtering.
  match_snippet?: string;
}

export interface TrafficResp {
  enabled: boolean;
  proxy?: string;
  count?: number; // global total (unfiltered)
  total?: number; // rows matching the current filter (for pagination)
  page?: number;
  size?: number;
  exchanges?: TrafficExchange[];
  status_buckets?: Record<string, number>; // Complete status distribution: {"2":N,"3":N,"4":N,"5":N}.
}

// Full raw request/response of one exchange (lazy-loaded on row select).
export interface TrafficDetail {
  req: string;
  resp: string;
}

// ---- App settings (runtime toggles) ----
export interface Settings {
  traffic_capture: boolean;
  // Web search. brave_key_set / tavily_key_set reflect whether a key is stored
  // (the values are never returned). On PUT, send the corresponding field to set/clear.
  web_search_enabled: boolean;
  web_search_backend: string; // "ddgs" | "brave-free" | "tavily"
  brave_key_set: boolean;
  tavily_key_set: boolean;
  // write-only: only sent on PUT to store/clear the key.
  brave_search_api_key?: string;
  tavily_search_api_key?: string;
  // Independent search-endpoint proxy, separate from recording MITM; empty=direct.
  web_search_proxy?: string;
  // Global outbound proxy as MITM upstream for agent target traffic; empty=direct.
  global_proxy?: string;
  python_interpreter?: string; // Custom-script Python path; empty=runtime detection.
  workers?: number; // Worker concurrency, default 3; applies to subsequently started tasks.
}

// ---- In-app updates ----
export interface UpdateCheck {
  current: string; // Running version; dev disables in-app updates.
  latest?: string; // Latest stable GitHub release tag.
  has_update?: boolean;
  comparable?: boolean; // Whether versions can be compared; dev/suffixed builds are excluded.
  reason?: string; // Why comparison is unavailable.
  mode: "binary" | "docker";
  os: string;
  arch: string;
  asset?: string; // Release archive for this platform.
  asset_available?: boolean; // Whether the release includes this platform.
  size?: number;
  notes?: string; // Release notes/changelog.
  html_url?: string;
  published_at?: string;
  has_backup?: boolean; // Previous version available for rollback.
  repo?: string;
  boot_notice?: string; // Last update result, such as rollback or verification failure.
  rolled_back?: boolean;
  error?: string; // Update-check failure, such as GitHub connectivity/rate limits.
}

export interface UpdateProgress {
  phase: "idle" | "downloading" | "verifying" | "extracting" | "staged" | "failed";
  percent: number; // Download progress 0–100; -1 for other stages.
  message: string;
  version?: string;
  error?: string;
}

// ---- LLM config ----
export interface LLMProfile {
  id: string;
  name: string;
  format: "openai" | "anthropic";
  base_url?: string;
  proxy?: string;
  model: string;
  api_key_hint?: string;
  rate_per_second: number;
  rate_per_minute: number;
  context_window_k?: number;
  // Thinking: empty=provider default, off=disabled, low/medium/high/max=enabled at selected effort.
  reasoning_effort?: string;
  is_default: boolean;
  priority?: number; // Failover priority descending; active profile always first.
  pool_exclude?: boolean; // Exclude from fallback selection.
  session_header_key?: string; // Optional request header carrying the current session ID.
}

// LLM failover member with circuit-breaker state.
export interface LLMPoolMember {
  profile_id: string;
  name: string;
  model: string;
  format: string;
  priority: number;
  active: boolean; // Global active profile, always first.
  excluded: boolean; // pool_exclude prevents fallback use.
  state: "ok" | "degraded" | "tripped";
  fails: number;
  trips: number;
  cooldown_secs: number; // Remaining circuit-breaker cooldown; 0=none.
  last_error?: string;
  last_at?: string;
}

export interface LLMPoolStatus {
  enabled: boolean;
  chain: LLMPoolMember[];
}

// ---- Agents ----
export interface Agent {
  id: string;
  key: string; // Built-in goals/planner/mainagent/worker or a user-defined key.
  name: string;
  description?: string;
  role: string;
  builtin: boolean;
  enabled: boolean;
  max_turns?: number; // 0 means unlimited.
  run_seconds?: number; // Worker per-run duration in seconds; 0 means unlimited.
  web_search?: boolean; // Agent search toggle, gated by the global setting.
  interactive_shell?: boolean; // Enable persistent interactive PTY tools.
  llm_profile_id?: number; // Role-bound profile; 0/absent uses global active profile.
  // Binding counts returned by list endpoints: MCP, skills, and tools.
  mcp_count?: number;
  skill_count?: number;
  tool_count?: number;
}

export interface PromptVar {
  name: string;
  description: string;
  example: string;
  source: "exploration" | "runtime" | "distilled";
}

export interface PromptVersion {
  version: number;
  ts: string;
  note: string;
  template_text: string;
}

export interface AgentDetail {
  agent: Agent;
  prompt: string;
  variables: PromptVar[];
  versions: PromptVersion[];
  visibility: { mcp: number[]; skill: string[] };
  wrapup_prompt?: string; // Saved settlement prompt; empty uses built-in.
  wrapup_default?: string; // Built-in placeholder/reset prompt.
  wrapup_max_turns?: number; // Saved settlement turns; 0 uses built-in.
  wrapup_max_turns_default?: number; // Built-in settlement turn limit for UI hints.
  // Task-timeout prompts appear only when task_timeout_wrapup_supported=true.
  task_timeout_wrapup_supported?: boolean;
  task_timeout_wrapup_prompt?: string;
  task_timeout_wrapup_default?: string;
  task_timeout_wrapup_max_turns?: number;
  task_timeout_wrapup_max_turns_default?: number;
}

// ---- MCP ----
export interface MCPServer {
  id: number;
  name: string;
  transport: "stdio" | "http";
  command?: string;
  args: string[];
  env: Record<string, string>;
  headers?: Record<string, string>; // http transport request headers
  url?: string;
  description?: string;
  timeout_seconds?: number;
  enabled: boolean;
  status?: string;       // '' | 'ok' | 'error'
  status_at?: string;    // ISO timestamp
  status_error?: string; // error message
  tools?: string[];      // mcp_tools_cache (names only, for the count)
}

// MCP usage statistics, analogous to SkillStat.
export interface MCPStat {
  server_name: string;
  calls: number;
  last_at?: string;
  by_tool: Record<string, number>;
  by_agent: Record<string, number>;
}
export interface MCPStatsResp {
  servers: MCPStat[];
  total_calls: number;
  enabled_count: number;
  total_count: number;
}

export interface MCPTool {
  name: string;
  description: string;
}

// ---- Skills ----
// Fields align with the agentskills.io open specification.
// description covers both "what the skill does" and "when to use it".
export interface SkillItem {
  name: string;           // unique key = directory name
  description?: string;   // required per spec; covers what + when to use
  license?: string;       // optional: SPDX identifier or free text
  compatibility?: string; // optional: environment requirements
  mcps?: string[];        // MCP server names this skill unlocks on load
  files: string[];        // files in the skill directory
}

// Skill usage statistics.
export interface SkillStat {
  skill_name: string;
  calls: number;
  last_at?: string;
  by_agent: Record<string, number>; // agent_key → calls
}
export interface SkillStatsResp {
  skills: SkillStat[];
  total_calls: number;
  installed_count: number;
  invoked_count: number;
}

// ---- Built-in tool catalog ----
// key + handler live in Go; only these fields are page-editable. system tools lock
// the key and the parameter *structure* (name/type/required) — the per-param
// description/default and the agent binding are what move.
export interface Tool {
  key: string;
  system: boolean;
  description: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  schema: Record<string, any>; // full JSON-Schema (object with properties)
  agents: string[];            // bound agent keys
  enabled: boolean;
  kind?: "builtin" | "shell" | "command" | "script" | "http"; // Custom tool kind.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  exec?: Record<string, any>;  // Execution specification for non-built-in tools.
  deferred?: boolean;          // Deferred schema through SearchExtraTools/ExecuteExtraTool.
}

// ---- Stats ----
export interface Stats {
  assets: number;
  engine_mode: EngineMode;
  llm_configured: boolean;
  roe_enabled: boolean;
  findings_confirmed: number;
}

// ---- Intercept Rules ----
export type InterceptAction = "allow" | "deny" | "ask";
export type InterceptMatchTarget = "tool_name" | "tool_input";
export type InterceptMatchType = "string" | "regex";

export interface InterceptRule {
  id: number;
  name: string;
  enabled: boolean;
  priority: number;
  match_target: InterceptMatchTarget;
  match_type: InterceptMatchType;
  pattern: string;
  action: InterceptAction;
  message: string;
  timeout_enabled: boolean;
  timeout_seconds: number;
  timeout_action: "deny" | "allow";
  created_at: string;
  updated_at: string;
}

export interface InterceptPending {
  id: number;
  rule_id?: number;
  conversation_id?: number;
  task_id?: string;
  agent_name: string;
  tool_name: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  tool_input: Record<string, any>;
  status: "pending" | "allowed" | "denied" | "timeout";
  decided_at?: string;
  created_at: string;
}

// InterceptApprovalRow enriches InterceptPending with conversation/task and rule context.
export interface InterceptApprovalRow extends InterceptPending {
  conv_title: string;      // "" if no linked conversation
  conv_agent_key: string;  // "" if no linked conversation
  rule_name: string;       // "" if rule was deleted
}

// ── ScopeSentry asset synchronization ───────────────────────────────────────
export interface SSProject {
  id: string;        // MongoDB ObjectID — used as filter.project
  name: string;
  logo?: string;
  AssetCount?: number;
  tag?: string;
}

export interface SSTask {
  id: string;
  name: string;      // used as filter.task
  status?: number;
  progress?: number;
  creatTime?: string;
  endTime?: string;
}

// ConvTokenSummary — one conversation's token total (+ profile/date) for merging
// chat usage into the dashboard token stats. GET /api/tokens/conversations.
export interface ConvTokenSummary {
  llm_profile_id: number | null;
  created_at: string;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
}

// ---- Platform RBAC ----
export interface RBACUser {
  id: string;
  username: string;
  display_name: string;
  enabled: boolean;
  is_builtin: boolean;
  created_at: string;
  updated_at: string;
  role_ids?: string[];
}

export interface RBACRole {
  id: string;
  name: string;
  description: string;
  scope: string;
  is_system: boolean;
  created_at: string;
  updated_at: string;
  permissions?: string[];
}

export interface RBACMe {
  user: { id: string; username: string; display_name: string };
  roles: RBACRole[];
  permissions: string[];
  scope: string;
}

export interface AuditLog {
  id: number;
  created_at: string;
  actor: string;
  category: string;
  action: string;
  result: string;
  ip: string;
  message: string;
  detail: Record<string, unknown>;
}

// Async .docx report generation job (AI narrative + python-docx pipeline).
export interface ReportJob {
  status: "idle" | "generating" | "done" | "failed";
  filename?: string;
  error?: string;
  started_at?: string;
}

// Report management: org-level settings + generated-report record.
export interface Personnel {
  name: string;
  role: string;
  contact: string;
}
export interface ReportSettings {
  company_name: string;
  company_name_en: string;
  logo_path: string;
  personnel: Personnel[];
  cvss_version: string;
  updated_at?: string;
}
export interface ReportRecord {
  id: number;
  task_id?: number;
  task_desc: string;
  filename: string;
  size: number;
  company: string;
  created_at: string;
}

export interface ReportTemplate {
  id: number;
  name: string;
  company_name: string;
  company_name_en: string;
  logo_path: string;
  personnel: Personnel[];
  cvss_version: string;
  locale: string;
  created_at?: string;
  updated_at?: string;
}

// ---- Global/task knowledge retrieval ----
export interface KnowledgeDoc {
  id: number;
  scope: string; // global | task
  task_id?: number;
  category: string;
  title: string;
  content?: string;
  source: string; // manual | upload | migrated
  file_name?: string;
  bytes: number;
  indexed: boolean;
  chunks: number;
  created_at: string;
  updated_at: string;
}

export interface KnowledgeHit {
  doc_id: number;
  title: string;
  category: string;
  scope: string;
  content: string;
  score: number;
}

export interface KnowledgeConfig {
  enabled: boolean;
  base_url: string;
  model: string;
  dim: number;
  has_key: boolean;
  vector_available: boolean;
  semantic_ready: boolean;
  docs_global: number;
  docs_task: number;
  api_key?: string; // write-only
}

export interface KnowledgeLog {
  id: number;
  task_id?: number;
  agent_key: string;
  query: string;
  scope: string;
  mode: string;
  top_k: number;
  hits: number;
  created_at: string;
}
