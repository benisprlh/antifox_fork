---
name: api-recon
description: Use this skill to discover a website's API endpoints.
---

# API Recon (Frontend API Reconnaissance)

With **authorization**, discover as much as possible of the **backend API** (paths, methods, parameters, response bodies), **frontend routes**, and **UI triggers** (tabs, dialogs, table actions, etc.).

---

## Boundaries and Prohibitions (Required Reading for Agents)

This skill covers **API and parameter reconnaissance only**, not vulnerability discovery or exploitation.

### Task Boundaries

| Scope | Allowed | Prohibited |
|---|---|---|
| **Objective** | Enumerate paths, methods, parameters, routes, and UI triggers | SQLi/XSS/access-control/brute-force/fuzz testing, request-tampering attacks, destructive operations |
| **Authentication** | Use hooks and stubs/mocks to bypass **client-side** login gates | Asking for or guessing credentials; submitting real login forms |
| **Runtime** | Hook APIs without credentials and use mock responses to render the SPA's authenticated shell | Workflows that require a real backend session to continue |

### Dynamic Analysis Without Credentials (Phase 3 Default)

1. **Intercept and stub** login, permissions, menus, and other bootstrap APIs with `preload.js` / `runtime_harvest.js`;
2. Return mock business-query responses with the **correct structure and successful status; data may be empty**;
3. Let the frontend render authenticated pages without a backend or despite 401 responses, triggering additional XHR/fetch/WebSocket requests;
4. **Empty data, blank tables, and placeholder UI are expected**; do not switch to real login or vulnerability testing to address them.

Use mocks to render frontend routes and mount components, and **record only outbound requests**. The objective is to learn which additional APIs the frontend calls, regardless of the backend response.

### Workflow Prohibitions

| Prohibited | Alternative |
|---|---|
| Using grep/curl/Read on the main `index-*.js` entry to extract API paths before Phase 1 finishes | Run `OUTDIR/harvest_static.py` |
| Writing replacement harvesting scripts such as `extract_apis.py` | Modify and rerun `OUTDIR/harvest_static.py` |
| Repeating the same grep/command after two or more failures | Change strategy: read tool_logs, modify harvest, consult the reference |
| Skipping gates A/B and running unmodified `scripts/` files | Copy to OUTDIR and adapt to the target |
| Real username/password, OTP, OAuth, or other authentication | Use stubs/mocks as described above |
| Skipping stubs to obtain real data through access-control or injection testing | Record outbound requests only; stay within reconnaissance |
| Irreversible operations such as deletion, sensitive-data export, or bulk writes | The same restriction applies to coverage-mode clicks |
| Claiming all pages and APIs were discovered without runtime analysis and dynamic enumeration | Meet the Definition of Done or document limitations |
| Claiming all parameters are known without the trigger matrix and comparison | Complete the Phase 3b matrix and Phase 5 comparison |
| Inferring required/optional fields from a single runtime sample | Compare multiple samples or infer from validation rules/errors |

---

## Two-Layer Model and Runtime Modes

| Layer | Output | Limitations |
|---|---|---|
| **Static** (JS bundles) | Endpoint paths, draft routes, candidate request fields | No HTTP methods; parameters require Phase 1b; runtime-built URLs may be missed |
| **Runtime** (live session) | Methods, bodies, responses, dynamic URLs, WS/SSE; multiple samples refine parameters | Pages must render to trigger requests; one sample cannot establish required/optional fields |

| Runtime Mode | Engine | Use Case |
|---|---|---|
| **depth** | `runtime_harvest.js` (Puppeteer) | API inventory, METHOD/params/response bodies, WS/SSE, repeatable batch runs |
| **coverage** | Browser + `preload.js` | Click tabs, dialogs, and tables for deeper feature coverage |
| **both** | depth followed by coverage | Most complete; takes the longest |

**Parameter methodology** (no universal script): Use harvesting/regex for paths; use **expanded context around anchors, UI binding chains, comparisons across samples, and error-based inference** for parameters (see section J of [reference.md](reference.md) for grep recipes).

---

## Definition of Done

All requirements must be met before declaring reconnaissance complete:

- [ ] **Static**: Phase 1 harvest produces `api_static.txt`, `routes.txt`, and `js/`
- [ ] **Runtime**: At least depth or coverage; coverage/both requires **working hooks and the dynamic enumeration loop**
- [ ] **Shell access**: Business paths render outside `/login` (check hash routing)
- [ ] **Parameters**: coverage/both completes the trigger matrix and `param_samples.json`; Phase 5 merges `params_merged.json`
- [ ] **Depth** (if module pages are blank): Reconstruct the permission tree in Phase 4 and rerun until **module-level APIs** appear, beyond locale/bootstrap
- [ ] **Delivery**: All Phase 5 outputs are present (see its output table); `insert_assets` records service and endpoint assets

---

## Scripts and Gates

`scripts/` contains reference templates only. **Do not** run them unchanged and treat the output as final.

**Rule**: Read first, adapt to the target, write into `OUTDIR` (such as `recon/`), and record changes in `CHANGES.md`. If unsuitable, rewrite using the methodology and retain only the structure.

| Gate | When | Reference Script → OUTDIR Copy | Common Adjustments |
|---|---|---|---|
| **A (static)** | After Phase 0, **before the first** harvest/spider run | `harvest_static.py` / `spider_mpa.py` | **Default regex works for most sites**; adjust endpoint patterns, webpack/Vite `publicPath`, or MPA exclusions/cookies only for manifest/dialect mismatches |
| **B (runtime)** | After Phase 2, before depth/coverage | `runtime_harvest.js` / `preload.js` + `config.json` | Cookie/localStorage keys, neutralize success value, stubs, login regex, API prefixes, hash/history |

**Required SPA order** (do not reorder; phase numbers take precedence over general exploration-first guidance):

| Step | Required | Prohibited |
|---|---|---|
| After Phase 0 | Next Bash command: `python3 OUTDIR/harvest_static.py <URL> OUTDIR` | curl/grep/Read of the main `index-*.js` entry (usually >500KB) |
| Gate A | Copy script, make small changes as needed, **run immediately** | Manually extracting APIs before deciding whether to harvest |
| Before Phase 1 completes | Validate with `wc -l`; adjust harvest and retry 404s | Writing extract scripts; repeatedly grepping URLs not downloaded |
| From Phase 1b | grep only `OUTDIR/js/*.js` | Substituting the main bundle for harvest |

- ✅ Copy `harvest_static.py`, optionally adjust regex, and **run immediately**
- ❌ curl the main bundle, grep repeatedly, write a temporary extractor, and harvest last
- **MPA**: After Phase 0, the next Bash command is `python3 OUTDIR/spider_mpa.py ...`

---

## Tool and Output Constraints

| Constraint | Details |
|---|---|
| Large files | **Do not** Read/grep `index-*.js` files >100KB into context; batch-process with OUTDIR scripts |
| grep output | Must use `\| head -20` or `-m 5`; retain path summaries only, not bundle excerpts |
| Validation | Use `wc -l` and `ls \| wc -l`; do not Read entire directories |
| Initial regex probe | Optional, at most once, only HTML or chunks ≤50KB; harvest remains the source for formal static analysis |
| Reference | Recipes, templates, and troubleshooting are in [reference.md](reference.md); do not duplicate it inline |

---

## Execution Roadmap

```
Phase 0 classification + OUTDIR
  → Gate A → Phase 1 harvest (★ run immediately ★)
  → Phase 1b parameter reverse engineering
  → Phase 2 three authentication gates → config.json
  → Gate B → Phase 3 runtime + parameter matrix
  → Phase 4 permission tree (if needed) → rerun Phase 3
  → Phase 5 merge/report + insert_assets: batch-insert every discovered service and API endpoint asset; do not omit any discovered assets
```

Check off each step in order. **Do not advance before completing the previous phase**.

1. [ ] **Phase 0**: Identify SPA/MPA; create `OUTDIR` → [Phase 0](#phase-0--classification)
2. [ ] **Gate A + Phase 1**: Copy script, harvest **immediately**, validate with `wc -l` → [Phase 1](#phase-1--static-analysis)
3. [ ] **Phase 1b**: Expand anchor context and trace bindings → `param_candidates.json` → [Phase 1b](#phase-1b--parameter-reverse-engineering)
4. [ ] **Phase 2**: Three authentication gates → `config.json` → [Phase 2](#phase-2--three-authentication-gates)
5. [ ] **Gate B**: Adapt runtime scripts → [Phase 3](#phase-3--runtime)
6. [ ] **Phase 3**: depth / coverage / both; verify shell access; parameter trigger matrix → `param_samples.json`
7. [ ] **Phase 4** (if needed): Permission tree, patch stubs, rerun Phase 3 → [Phase 4](#phase-4--permission-tree-reconstruction)
8. [ ] **Phase 5**: Merge outputs, report, and `insert_assets` → [Phase 5](#phase-5--merge-and-report)

---

## Phase 0 — Classification

Fetch entry HTML and **create `OUTDIR`** (do not modify the skill's `scripts/`):

- **SPA**: Empty shell + `<div id=app>` + chunks → Phases 1–5
- **MPA**: SSR + `<form>`, no endpoint bundle → after gate A:

```bash
python3 recon/spider_mpa.py <BASE_URL> <OUTDIR> [--cookie "session=..."] [--max 300] [--depth 5] [--exclude "logout|delete|destroy"]
```

Produces `forms.txt`, `links.txt`, and `api_inline.txt`. For an SPA with approximately zero forms, switch to Phase 1.

---

## Phase 1 — Static Analysis

Follow [Scripts and Gates](#scripts-and-gates) and [Tool and Output Constraints](#tool-and-output-constraints).

```bash
python3 recon/harvest_static.py <BASE_URL> <OUTDIR>
```

harvest: Parse HTML scripts, resolve webpack/Vite manifests, download all lazy chunks, and produce `js/`, `api_static.txt`, `routes.txt`, and `chunkmap.txt`.

```bash
wc -l OUTDIR/api_static.txt OUTDIR/routes.txt
ls OUTDIR/js | wc -l
```

- Compare chunk count with the manifest: adjust harvest and retry 404s instead of curling chunks individually
- Too few entries in `api_static.txt`: broaden the endpoint regex in OUTDIR and rerun (see reference)

### Phase 1b — Parameter Reverse Engineering

Paths come from Phase 1; parameter fields need separate reconnaissance. Follow [Tool and Output Constraints](#tool-and-output-constraints) for grep.

**Completion criteria**: For important endpoints, identify field names, transport locations, inferred types, required status, sample values, and confidence.

#### 1b.0 — Transport Format

| Format | Parameter Location | Static Clues |
|---|---|---|
| REST JSON | body + query | `(params\|data\|body)\s*:\s*\{` near path anchors |
| GraphQL | `variables` | gql templates, `$page: Int` |
| Traditional form | urlencoded | `<form>`, `FormData` |
| File upload | multipart | `FormData.append` |
| Path parameters | `/user/:id` | Route tables + `useParams` / `$route.params` |
| Encryption/signing | Wrapped in `sign`/`data` | Hook encryption-function arguments (reference section D) |

Output: Label each endpoint with `transport: query|json|form|graphql|encrypted`.

#### 1b.1 — Expand Context Around Anchors

Use a known path as an anchor and expand the surrounding context to find request-building objects:

```bash
grep -n '"/api/user/list"' OUTDIR/js/*.js | head -20
grep -rhoaE '.{0,120}("/api[^"]+").{0,200}' OUTDIR/js/*.js | head -20
grep -rhoaE '(params|data|body|payload)\s*:\s*\{' OUTDIR/js/*.js | head -20
```

| Wrapper | Parameter Clues |
|---|---|
| axios instance | `data` / `params` |
| Shared request wrapper | Interceptors injecting global fields |
| OpenAPI client | Generated method signatures |
| React Query / SWR | Second hook argument |
| Vue composable | Composable arguments |

Type clues: `yup`/`zod`/rules, `Form.Item name=`, embedded Swagger.

→ `param_candidates.json`：`{ path, fields[], source: "static-callsite", confidence }`

#### 1b.2 — Binding Layer

```
Form field → onFinish/handleSubmit → transform → API payload
```

| Binding Source | Technique |
|---|---|
| Form submit | Trace submit → transform → API |
| Table search | `getFieldsValue()` → `params` |
| Route | `:id` / `?tab=` |
| Interceptor | Global `tenantId`, pagination, sign |
| Enum select | `options` → API enumeration values |

Trace upward in the DevTools call stack from `fetch`/`XHR.send` to the request-building function.

#### 1b.3 — Three Request Questions (Distinct from Phase 2 Authentication Gates)

| Question | What to Determine |
|---|---|
| **Assembly** | Where the payload is built and transformed |
| **Validation** | required, pattern, enum |
| **Transport** | path / query / body / multipart / headers |

When inspecting the interceptor gate (Phase 2), also read globally injected fields such as Authorization, `X-Tenant-Id`, and sign.

#### 1b.4 — Connection to Phase 3

Candidate fields come from static analysis/bindings; **required, optional, and conditional fields** require the Phase 3 trigger matrix, sample comparisons, and Phase 5 error-based inference.

---

## Phase 2 — Three Authentication Gates

grep in `OUTDIR/js/` (with `head`) and write findings to `config.json` (recipes in reference):

| Gate | Question | Keywords |
|---|---|---|
| **Rendering gate** | How is login status determined? | `isLogin`, `getToken`, Cookie/localStorage |
| **Interceptor gate** | What triggers a redirect to `/login`? | `response_code`, `errno`, axios interceptor |
| **Content gate** | Where do menus/permissions come from? | `menu`, `permission`, `role`, `acl`, `routes` |

Do not treat localStorage key names as credentials; confirm their purpose from chunks and request chains.

**Exit = gate B**: Record conclusions in `config.json` and adapt `OUTDIR/runtime_harvest.js` / `preload.js`.

### Phase 2b — API Observation (Optional)

Use `preload.js` in OUTDIR to confirm session keys, Authorization, and nested API URLs:

| Setting | Output |
|---|---|
| `recordDetail: true` | `__API_RECON_DETAIL__` |
| `observe.xhrHeaders: true` | Header observations |
| `extractUrlsFromResponse: true` | Nested API URLs in responses |
| `observe.storageReads/cookieReads: true` | Values to add to config |
| `neutralizeVueRouter: true` | `__API_RECON_ROUTES__` |

Export each coverage round: `__API_RECON_LOG__`, `__API_RECON_DETAIL__`, `__API_RECON_ROUTES__`, `__API_RECON_OBSERVE__`.

---

## Phase 3 — Runtime

Gate B must be complete. Follow [Boundaries and Prohibitions](#boundaries-and-prohibitions-required-reading-for-agents) and the credential-free mock strategy.

Set `"runtimeMode": "depth" | "coverage" | "both"` in `config.json` (template in reference).

### Hooks and Stubs (Shared by depth and coverage)

| Layer | Scope | Purpose |
|---|---|---|
| L1 exact | auth/permissions/bootstrap stubs | Pass initial authentication checks |
| L2 error correction | All JSON responses | Convert unauthenticated status to success |
| L3 fallback | `/api` and similar requests not matched by L1 | Empty successful bodies to render the UI |

- **depth**: Fake auth + `forward` to rewrite business status codes + `stubs`; traverse `routes` (hash/history); produce `runtime_api.json`
- **coverage**: Inject `preload.js` at **document-start** (CDP `addScriptToEvaluateOnNewDocument` or Userscript)

Verify that `window.__API_RECON_PRELOAD__` exists and business paths do not return to `/login`.

```bash
cd recon && npm install
node runtime_harvest.js config.json
```

### 3b — Dynamic Enumeration in coverage Mode (Required)

1. Main navigation/sidebar: Click every item and wait 1–3 seconds for network activity
2. Tab — `role=tab`、`.ant-tabs-tab`
3. Tables: Open view/edit/details for the first row
4. Toolbars: Export, filter, create (**avoid irreversible deletion**)
5. On entering each module: Merge APIs/routes
6. SPA: Use controlled `pushState` for paths not covered by `routes.txt` (prohibited for MPA)

**Parameter trigger matrix** (required): Record each action type in every module and **compare multiple samples**:

| Action | Common Additional Parameters |
|---|---|
| Initial list view | Pagination + default filters |
| Search | keyword, filter |
| Advanced filters | More optional fields |
| Create/edit | Complete entity |
| Bulk/export/sort | `ids[]`, `exportType`, `sortField` |

**Outbound bodies/headers remain real when responses are stubbed**; use the requests as evidence. Record into `scan_raw.json`, `param_samples.json`, and `api_detail.json`.

- **Vue**：`neutralizeVueRouter: true` + document-start preload
- **React**: `routes.txt` + sidebar clicks + `pushState`
- **both**: Run 3a depth, then 3b coverage

---

## Phase 4 — Permission Tree Reconstruction

**Trigger**: Blank module pages or only bootstrap requests (such as locale) on every route indicate the content gate has not been passed.

| Symptom | Meaning |
|---|---|
| Shell renders successfully | Rendering and interceptor gates passed |
| Missing sidebar items/blank views after clicking | Incomplete stub shape or permission codes |
| Very few identical APIs on every route | `v-if permission` check failed |
| Far fewer entries in `routes.txt` than in bundles | Complete routes from the auth module |

```bash
grep -rhoaE '"/api[^"]*(permission|perm|role|menu|acl)[^"]*"' OUTDIR/js/*.js | sort -u | head -30
grep -rhoaE 'userRouteAuth|getResultTree|routeMap|routeLink|menuList|authList' OUTDIR/js/*.js | head -20
```

Typical chain: `role_permissions` (flat codes) + `permissions/all` (tree) → `getResultTree` → `userRouteAuth[CODE].url`.

```bash
python3 recon/extract_route_map.py recon/js recon/
python3 recon/build_perm_tree.py recon/js recon/ --config recon/config.json
```

Intermediate outputs: `route_map.json`, `userRouteAuth.json`, `permissions_tree.json`, `*_stub.json`, `perm_codes_all.txt`.

Check stubs: Outer `response_code` matches the interceptor gate; flat codes align with the tree; `routes` includes every link from `route_map`.

After updating `config.json`, **rerun Phase 3**. For large SPAs, adjust `waitUntil`, `routeTimeout`, and `perRouteMs` (reference sections A3/I).

---

## Phase 5 — Merge and Report

### Outputs

| File | Phase | Contents |
|---|---|---|
| `js/`, `api_static.txt`, `routes.txt`, `chunkmap.txt` | 1 | Static bundles and paths |
| `param_candidates.json` | 1b | Candidate fields from static analysis |
| `config.json` | 2 | Three gates and runtime configuration |
| `runtime_api.json` | 3a | Detailed depth recording, including WS/SSE |
| `param_samples.json`, `scan_raw.json`, `api_detail.json` | 3b | Multiple samples, click logs, details |
| `route_map.json`, etc. | 4 | Intermediate permission-tree files, if this phase ran |
| `params_merged.json` | 5 | Merged parameter fields and confidence |
| `api_merged.txt` | 5 | `METHOD /path [params] [static\|runtime\|both]` |
| `site_map.json` | 5 | Routes, APIs, parameters, features, limitations |
| **insert_assets** | 5 | Write all service and endpoint assets to the asset inventory |

### 5b — Merge Parameters

Compare samples in `param_samples.json`; **there is no universal merge script**. Confidence rules are in reference section J7 (high/medium/low/pending trigger).

### 5c — Error-Based Inference

Within the authorized scope, send incomplete requests to inspect 400 responses (**parameter reconnaissance, not vulnerability testing**), such as `field 'x' is required` or enum errors. Account for `data` wrappers, `variables`, and pre-encryption `bizData`.

The report must state runtimeMode, static/runtime API counts, parameter confidence, uncovered modules, and a `CHANGES.md` summary of modifications to reference scripts.

Suggested `site_map.json` structure:

```json
{
  "site": "https://example.com",
  "runtimeMode": "both",
  "appType": "vue-spa",
  "routeGuardStrategy": ["nav-neutralize", "L1-auth", "L2-patch", "forward"],
  "apisFromStatic": [],
  "apisFromRuntime": [],
  "apis": [],
  "params": [{ "method": "POST", "path": "/api/user/list", "transport": "json", "fields": [] }],
  "frontendRoutes": [],
  "routesVerifiedByClick": [],
  "featuresTriggered": [],
  "limitations": ""
}
```

See [reference.md](reference.md) for additional fields and grep recipes.

---

## General Notes

- **Framework independent**: The same methods apply to webpack/Vite/Angular lazy loading
- **Transport**: REST/JSON, GraphQL, WebSocket, SSE; gRPC-web is out of scope
- **SSR**: Client fetches can be recorded; RSC/Server Actions cannot be fully enumerated
- **Blind spots**: JSVMP, WASM, strong HMAC/mTLS validation → use static analysis and document limitations
- **Parameter blind spots**: Conditional dependencies, hidden parameters, WASM request building → mark as "pending trigger" or "unreachable"
- **Static analysis is the fallback**: Endpoints can still be enumerated when runtime access is blocked

---

## Additional Resources

- Grep recipes, `config.json` templates, troubleshooting, hooks, parameter reverse engineering (section J), and site_map templates: **[reference.md](reference.md)**
- Reference script paths are listed in [Scripts and Gates](#scripts-and-gates)
