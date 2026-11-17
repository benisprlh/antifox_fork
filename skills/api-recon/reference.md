# api-recon — Reference Manual

Grep recipes, `config.json` templates, and troubleshooting. Run all grep commands against `js/`. For single-line bundles, optionally use `js-beautify` or `sed 's/}/}\n/g'`; raw grep with surrounding context is usually sufficient.

## Script Notes

All files in `scripts/` are **reference templates** and must be adapted to the target before execution. Typical changes:

| Script | Common Adjustments |
|---|---|
| `harvest_static.py` | Endpoint regex, webpack/Vite manifest parsing, microfrontend publicPath, retries/concurrency |
| `runtime_harvest.js` | neutralize field names/success values, stub matching/body shape, route sources, WS recording, `waitUntil`/`routeTimeout`/`proxy` |
| `preload.js` | `loginPathRe`, L1 stubs, `neutralize.fields`, `apiPattern`, L3 enablement, `recordDetail`, `observe.*`, `neutralizeVueRouter` |
| `spider_mpa.py` | `--exclude` for destructive links, cookies, depth/max, same-domain filtering |
| `extract_route_map.py` | `routeMap` / `routeLink` regex, KEY naming patterns |
| `build_perm_tree.py` | `userRouteAuth` parsing, `ROOTS`/`PREFIX_PARENT` hierarchy heuristics, outer stub field names |
| `config.json` | Central location for all target-specific settings above |

Keep adapted files in the task's working directory (such as `recon/`) and document specific changes from the reference scripts in the report.

---

## A. Reverse Engineer the Three Gates

### A1. Rendering Gate — How Is Login Status Determined?

```bash
grep -rhoaE '.{0,40}(isLogin|isAuthenticated|loggedIn|hasLogin|requireAuth)\b.{0,80}' js | head
grep -rhoaE 'function (getUser|getToken|getAuth)[0-9]?\([^)]*\)\{.{0,200}' js | head
grep -rhoaE '(localStorage|sessionStorage)\.getItem\("[^"]+"\)' js | sort -u
grep -rhoaE '(Cookies?|cookie)\.(get|load)\("[^"]+"\)' js | sort -u
grep -rhoaE '\batob\(|JSON\.parse\(|jwt|decode' js | head
```

Trace `isLogin = f(getUser())` → `getUser = decode(storage.read(KEY))` to identify the **storage key**, **container** (Cookie vs localStorage), and **encoding**:

| Encoding | How to Mock It in config |
|---|---|
| Plain string / `"1"` / token | `"value": "anything-truthy"` |
| `JSON.parse(x)` | `"value": "json:{\"id\":1,\"username\":\"admin\"}"` |
| `JSON.parse(atob(x))` | `"value": "b64json:{\"id\":1,\"username\":\"admin\"}"` |
| JWT | Unsigned/`alg:none` JWT, or signed with a key found in the bundle |
| Encryption (SM2/AES/RSA) | Look for hardcoded keys; mock when the rendering gate needs only a decodable blob, otherwise fall back to static analysis |

→ Write into `cookies` / `localStorage`.

### A2. Interceptor Gate — What Triggers a Redirect to /login?

```bash
grep -rhoaE '.{0,60}(interceptors\.response|axios|request\.use).{0,120}' js | head
grep -rhoaE '.{0,40}(response_code|errcode|errno|\bcode\b|\bret\b|\bstatus\b)\s*[=!]==?\s*[\-0-9]{1,4}.{0,60}' js | head -20
# Match the original Chinese login-error strings as well as English errors.
grep -rhoaE '.{0,40}(未登录|请重新登录|登录已过期|unauthorized|登录失效|授权|token.{0,10}invalid).{0,40}' js | head
grep -rhoaE '.{0,30}(location\.href|router\.(push|replace)|navigate)\([^)]*login[^)]*\)' js | head
```

Identify the **field name**, **success value** (usually `0` or `200`), and **failure values that trigger redirects**. Verify with a junk session:

```bash
curl -sk -X POST -H 'Cookie: <fakekey>=junk' https://target/api/<protected> -d '{}' -H 'Content-Type: application/json'
```

→ Write into `neutralize.fields` + `neutralize.success`.

### A3. Content Gate — Where Do Menus and Permissions Come From?

```bash
grep -rhoaE '"/api[^"]*(permission|perm|role|menu|acl|resource|nav)[^"]*"' js | sort -u
grep -rhoaE '.{0,30}(menus|permissions|menuList|routeList|authList|role_permissions)\b.{0,120}' js | head
grep -rhoaE 'userRouteAuth|getResultTree|routeMap|routeLink|hasPermission|checkAuth' js | head
grep -rhoaE '([A-Z_][A-Z0-9_]*):\{name:"[^"]*",link:"/[^"]+"\}' js | head
```

**Two data layers** commonly found in enterprise admin interfaces:

| API | Typical Payload | Consumer |
|---|---|---|
| `.../role_permissions` | `{ permissions: string[], role_type }` | Route guards, button-level ACL |
| `.../permissions/all` | `tree[{ code, position, children }]` | Sidebar menu rendering |
| `userRouteAuth` in bundle | `{ CODE: { url, name? } }` | code → frontend path |
| `routeMap` in bundle | `{ KEY: { name, link } }` | Alias resolution (webpack `o.DASHBOARD`) |

Read consumer code to confirm how `getResultTree(tree, permissions)` filters entries and which fields `v-if` / `hasAuth(code)` checks.

**Manual mocking** (small sites): Build a permissive payload → `stubs`.

**Full permission-tree reconstruction** (large sites with blank sidebars/submodules): See **section I**.

---

## B. config.json Template

```json
{
  "baseUrl": "https://target/",
  "runtimeMode": "both",
  "chromium": "/usr/bin/chromium",

  "cookies": [
    { "name": "auth", "value": "b64json:{\"id\":1,\"username\":\"admin\",\"role\":\"admin\",\"func\":{},\"permissions\":[\"*\"]}" }
  ],
  "localStorage": { "token": "faketoken", "isLogin": "1" },

  "neutralize": {
    "fields": ["response_code", "code", "errno", "ret", "status"],
    "success": 0,
    "flags": { "success": true, "message": "ok" }
  },
  "forward": true,
  "loginUrlPattern": "/login",
  "apiPattern": "/api/|/rest/|/graphql",

  "mockTier": "L1+L2",
  "recordDetail": true,
  "observe": {
    "storageReads": false,
    "cookieReads": false,
    "xhrHeaders": true
  },
  "neutralizeVueRouter": true,
  "stubs": [
    {
      "match": "permissions/all|/menu|role_permissions",
      "body": {
        "response_code": 0, "code": 0,
        "data": {
          "permissions": ["*"],
          "menus": [
            { "name": "dashboard", "path": "/dashboard", "show": true, "children": [] },
            { "name": "alert", "path": "/alert", "show": true, "children": [] }
          ]
        }
      }
    }
  ],

  "explore": {
    "clickTabs": true,
    "clickTables": true,
    "pushStateFallback": true,
    "maxMenuItems": 50
  },

  "routes": ["/dashboard", "/alert", "/asset", "/device", "/report", "/config", "/system"],
  "waitMs": 1500, "perRouteMs": 900, "headless": true,
  "waitUntil": "domcontentloaded",
  "routeTimeout": 12000,
  "proxy": "",

  "captureResponses": true, "recordWs": true, "respMax": 600
}
```

Field descriptions:
- `runtimeMode`：`depth`（Puppeteer）、`coverage`（browser MCP）、`both`
- `cookies[].value` prefixes: `b64json:` → base64(JSON); `json:` → raw JSON; no prefix → literal value
- `forward: true` forwards real requests and rewrites status fields; `false` uses fully offline stubs
- `mockTier`: Preload layers enabled in coverage mode, such as `L1+L2` or `L1+L2+L3`
- `routes` comes from `routes.txt`; the harness automatically adds `<a href>` links after menus are mocked
- `captureResponses` / `recordWs` apply only to depth mode
- `waitUntil`: Use `domcontentloaded` for large SPAs to avoid hangs with `networkidle2`
- `routeTimeout`: Per-route `page.goto` timeout in milliseconds
- `proxy`: Puppeteer `--proxy-server`; alternatively set `HTTP_PROXY` / `HTTPS_PROXY`

### B1. Dual-Stub Template (role_permissions + permissions/all)

```json
"stubs": [
  {
    "match": "role_permissions",
    "body": {
      "response_code": 0,
      "data": {
        "permissions": ["MONITOR", "MONITOR_ALERT", "THREAT", "ASSETS_RISK"],
        "role_type": "SUPER_ADMIN"
      }
    }
  },
  {
    "match": "permissions/all",
    "body": {
      "response_code": 0,
      "data": [
        {
          "code": "MONITOR",
          "position": 1,
          "children": [
            { "code": "MONITOR_ALERT", "position": 1, "children": [] }
          ]
        }
      ]
    }
  }
]
```

Outer field names (`response_code` / `code` / `data`) must match the A2 interceptor gate; `permissions` must include all leaf codes in the tree.

---

## C. coverage Mode: Preload Configuration

Edit the `CONFIG` object at the top of `scripts/preload.js`, or replace it before CDP injection:

```javascript
const CONFIG = {
  loginPathRe: /\/(login|signin)(\/|$|\?)/i,
  mockTier: 'L1+L2',
  forward: true,
  recordDetail: true,
  extractUrlsFromResponse: true,
  neutralizeVueRouter: true,
  observe: { storageReads: false, cookieReads: false, xhrHeaders: true },
  neutralize: { fields: ['response_code', 'code'], success: 0 },
  stubs: [ /* Same stubs as config.json */ ],
  apiPattern: /\/(api|apis|v\d+|dev|internal|graphql)\//i,
};
```

Verify `window.__API_RECON_PRELOAD__ === true` and a stable pathname.

Export recordings:

```javascript
JSON.stringify({
  apis: [...window.__API_RECON_LOG__],
  detail: window.__API_RECON_DETAIL__,
  routes: [...(window.__API_RECON_ROUTES__ || [])],
  observe: window.__API_RECON_OBSERVE__,
}, null, 2)
```

---

## D. preload / runtime Hook Capabilities

Built-in browser hooks and coverage in preload (coverage) and runtime_harvest (depth):

| Hook Capability | Value for API Discovery | Coverage |
|---|---|---|
| Hook fetch / XHR.open | Record request URLs/methods | ✅ `recordDetail` + `__API_RECON_LOG__` |
| Hook XHR.setRequestHeader | Discover Authorization and other headers | ✅ `observe.xhrHeaders` |
| Hook localStorage/cookie reads | Confirm session key names | ⚠️ Optional `observe.storageReads/cookieReads` |
| Extract Vue routes | Complete frontendRoutes | ✅ `__API_RECON_ROUTES__` (loaded routes) |
| Neutralize Vue route guards / block login redirects | Render modules to trigger APIs | ✅ `neutralizeVueRouter` + native redirect neutralization |
| Extract React routes | Add routes | ⚠️ Static analysis + clicks; no dedicated hook |
| Block page navigation to login paths | Keep pages open for analysis | ⚠️ Block login paths only to preserve business navigation |
| Hook encryption libraries (CryptoJS/SM, etc.) | Encrypted parameters → plaintext API body | ❌ Manually hook encryption-function arguments; record conclusions in config |
| Anti-debugging bypass | Required for runtime API recording on affected sites | ❌ Manual handling; static analysis still works |

---

## E. Endpoint Extraction Regex (When Static Results Are Sparse)

Broaden `extract_endpoints` in `harvest_static.py`, or inspect manually:

```bash
grep -rhoaE '"/[a-z][A-Za-z0-9_/\-]{3,}"' js | sort -u
grep -rhoaE '/api/[a-zA-Z0-9_./-]+' js | sort -u
```

---

## F. Troubleshooting

| Symptom | Cause → Action |
|---|---|
| Few static APIs | Endpoint dialect mismatch → broaden regex (section E) |
| Chunk count ≪ manifest | CSS-only or undeployed chunks; 404s have been retried |
| Runtime still shows login | Rendering gate incorrect → recheck A1 keys, container, encoding, domain |
| Shell renders but modules are blank | Content gate → mock menus (A3); `routes` paths may be wrong |
| Only bootstrap/locale on each route | Incomplete permission codes → reconstruct tree (I); check both `role_permissions` and `permissions/all` stubs |
| Sidebar items exist but subpages are blank | Missing intermediate tree nodes or codes inconsistent with `userRouteAuth` |
| Every API redirects to login | Interceptor gate → verify `neutralize`; extend traversal for nested fields |
| Zero WS frames | Subscription may require interaction; increase `perRouteMs` |
| Empty response bodies | Real responses exist only with `forward: true` |
| Missing Chromium | Install chromium or set `config.chromium` / `CHROMIUM` |
| Many mocks but still redirected to login | Hook too late or missing `location.href` setter → document-start + preload |
| All lists empty | L3 empty arrays are expected; continue to tabs/settings/details |
| Redux action mistaken for route | Filter internal paths containing get/set/change/clear/toggle/upload |
| Vue still redirects to login | Inject preload at document-start; or manually clear guards when `neutralizeVueRouter: false` |
| Response URL missing from log | Enable `extractUrlsFromResponse` or extract manually from `__API_RECON_DETAIL__` |
| Unknown Authorization header name | Enable `observe.xhrHeaders` or inspect request headers in DevTools |
| Runtime very slow / timing out | Use `waitUntil: domcontentloaded`, reduce `routeTimeout`, avoid `networkidle2` |
| Proxy connection failure | Check `proxy` / environment variables; use the same proxy port for Puppeteer and curl |

---

## G. Hardened Targets

When the server continuously validates sessions (unforgeable signed cookies or server-rendered menus that cannot be stubbed), runtime analysis may stop at the shell. Expected behavior:

- **Static analysis can still enumerate endpoints**: Module paths are present in code
- If authorization permits, run the same harness with a **real session**, `forward: true`, and no neutralize to capture real methods/parameters/responses

---

## H. Per-Task Checklist

1. Confirm the authorized scope
2. **Read** `scripts/harvest_static.py`, adapt it, run it, and review `api_static.txt` / `routes.txt`
3. **Phase 1b**: Expand path-anchor context and trace bindings → `param_candidates.json` (section J)
4. Reverse engineer A1/A2/A3 and write target-specific `config.json`
5. **Read and adapt** `runtime_harvest.js` / `preload.js` before execution
6. `runtimeMode=depth`: Run `npm install`, then the adapted harvest script
7. `runtimeMode=coverage/both`: Inject adapted preload at document-start; run browser MCP dynamic enumeration and the **parameter trigger matrix**
8. Modules fail to render: Reconstruct the permission tree (**section I**), patch stubs, and rerun
9. Compare multiple parameter samples and infer from errors → `params_merged.json`
10. Merge into `site_map.json` + `api_merged.txt`; accurately document coverage, gaps, and script changes

---

## I. Permission Tree Reconstruction (Phase 4 Details)

Use when a simple mocked `menus: [{ path, show: true }]` is insufficient and submodules still do not mount.

### I1. Locate the Auth Module

```bash
grep -l 'userRouteAuth' js/*.js
grep -l 'routeMap\|routeLink' js/*.js
grep -rhoaE 'getResultTree|role_permissions|permissions/all' js | head
```

Record **permission API paths**, **response field names**, and **consumer chunk filenames**.

### I2. Extract routeMap

```bash
python3 scripts/extract_route_map.py recon/js recon/
# Produces recon/route_map.json
```

If `[!] no routeMap pattern found`, broaden the regex in `extract_route_map.py` or grep manually:

```bash
grep -rhoaE '([A-Z_][A-Z0-9_]*):\{name:"[^"]*",link:"/[^"]+"\}' js | head -20
```

### I3. Build the Permission Tree and Stubs

```bash
python3 scripts/build_perm_tree.py recon/js recon/ --config recon/config.json
```

Script logic:
1. Parse `userRouteAuth={MONITOR:{url:...},...}`, including webpack aliases such as `He=o.DASHBOARD`
2. Resolve aliases to actual paths using `route_map.json`
3. Infer parents from code prefixes (`MONITOR_ALERT` → `MONITOR`)
4. Output `permissions_tree.json`, `permissions_all_stub.json`, and `role_permissions_stub.json`
5. With `--config`, automatically write `stubs` and expanded `routes` into `config.json`

**Adapt to the target** at the top of the script:
- `DEFAULT_ROOTS`: Top-level module codes
- `DEFAULT_PREFIX_PARENT`: `PREFIX_` → parent mapping
- `DEFAULT_EXTRA_PARENT`: Orphan nodes without prefix-based parent relationships

### I4. Validate Stub Consistency

```bash
# Permission count should approximately equal the number of userRouteAuth entries
wc -l recon/perm_codes_all.txt
# routes should include all route_map links
python3 -c "import json; m=json.load(open('recon/route_map.json')); r=set(json.load(open('recon/config.json'))['routes']); print('missing', [v['link'] for v in m.values() if v['link'] not in r])"
```

### I5. Rerun Runtime Analysis and Compare

```bash
node recon/runtime_harvest.js recon/config.json
# Compare runtime_api.json counts before/after mocking; check for module APIs under /attack, /asset, etc.
```

| Before Mocking | After Successful Mocking |
|---|---|
| Same 3–5 bootstrap requests per route | Different routes trigger different module APIs |
| Only `/api/locale/language` | Module endpoints such as `/api/web/...` appear |
| Single-digit routes in `routes.txt` | 80–110+ `routes` from route_map |

### I6. If It Still Fails

- **coverage mode**: Click sidebar items and tabs; permission-gated requests may require interaction
- **Stub fields**: Compare nesting in real APIs (curl + real session) and stubs
- **Additional guards**: grep button-level checks such as `hasPermission|checkRole|func.` and extend `role_permissions.permissions`
- **Static fallback**: Module paths remain in `api_static.txt`; runtime adds METHOD/body. Retain `param_candidates.json` and recorded samples

---

## J. Parameter Reverse Engineering (Phases 1b / 5b / 5c)

**Methodology, not a universal script.** Use regex for paths; use expanded anchor context, UI binding chains, multi-sample comparisons, and error-based inference for parameters.

### J1. Expand Anchor Context — Find Request Objects from Paths

```bash
# Use known Phase 1 paths as anchors
grep -n '"/api/user/list"' js/*.js
grep -rhoaE '.{0,120}("/api[^"]+").{0,200}' js | head
grep -rhoaE '(params|data|body|payload)\s*:\s*\{' js | head
grep -rhoaE '(get|post|put|delete|patch)\([^,]+,\s*\{' js | head
```

### J2. Wrappers and Transport Formats

```bash
# axios / shared request wrapper
grep -rhoaE '(axios|request)\.(get|post|put|delete|patch)\(' js | head
grep -rhoaE 'interceptors\.(request|response)' js | head

# GraphQL
grep -rhoaE '(query|mutation)\s+\w+|gql`|graphql\(' js | head
grep -rhoaE '\$[a-zA-Z_]+\s*:\s*(Int|String|Boolean|\[)' js | head

# FormData / multipart
grep -rhoaE 'FormData|\.append\(' js | head

# Path parameters
grep -rhoaE 'path:\s*"/[^"]*:[^"]+"' js | head
grep -rhoaE 'useParams|route\.params|\$route\.params' js | head
```

### J3. Validation Gate — Required Fields / Formats / Enums

```bash
grep -rhoaE '(required|message|pattern|enum|validator)\s*:' js | head
grep -rhoaE 'yup\.|zod\.|async-validator|Form\.Item|a-form-item|el-form-item' js | head
grep -rhoaE 'rules\s*:\s*\[|name:\s*["\'][a-zA-Z_]+["\']' js | head
grep -rhoaE 'label.*value|options\s*:\s*\[' js | head
```

### J4. Binding Layer — Forms → APIs

```bash
grep -rhoaE 'onFinish|handleSubmit|getFieldsValue|validateFields' js | head
grep -rhoaE '(pick|omit|transform|dayjs|moment)\(' js | head
```

Runtime support: DevTools → Network → request → **Initiator** (call stack); trace upward from `fetch`/`send` to the request-building function.

### J5. Encrypted Parameters

```bash
grep -rhoaE 'encrypt|decrypt|sign|CryptoJS|sm2|sm3|sm4|RSA|AES' js | head
```

**Do not guess fields from ciphertext**. Hook encryption-function **arguments** and record plaintext payloads before encryption; write conclusions to `config.json` / `param_candidates.json`.

### J6. Parameter Trigger Matrix (Required in Phase 3)

Record each action in every module and compare request bodies/queries:

| Action | Focus |
|---|---|
| Initial list view | Pagination defaults |
| Search | keyword, filters |
| Advanced filters | Optional fields |
| Create/edit | Complete entity |
| Bulk/export | `ids[]`, `exportType` |
| Sort/paginate | `sortField`, `order` |

Produce `param_samples.json`: `[{ "path", "method", "action": "search", "body", "query", "headers" }]`

### J7. Confidence Rules

| Confidence | Criteria |
|---|---|
| **High** | Static callsite agrees with at least two runtime samples |
| **Medium** | Static only, or only one runtime sample |
| **Low** | Inferred from responses/errors without further verification |
| **Pending trigger** | Field known statically, but UI/permissions have not reached it |

### J8. Scenario Quick Reference

| Scenario | Sequence |
|---|---|
| REST list page | J1 request object → J6 four comparisons → J3 rules |
| Create/edit form | J3 Form name → J4 submit chain → runtime submit and deliberately empty fields to inspect 400 responses |
| GraphQL | J2 variable declarations → record runtime variables for each operation |
| Encrypted body | J5 hook arguments → pre-encryption fields are the actual parameters |

### J9. Mapping to api-recon Phases

| api-recon | Parameter Reconnaissance |
|---|---|
| Phase 1 static | J1 expanded anchor context |
| Phase 2 A2 interceptor | Globally injected fields (tenantId, sign) |
| Phase 3 runtime | J6 trigger matrix + `param_samples.json` |
| Phase 4 permission tree | Forms differ by module; sufficient permissions are needed to trigger all fields |
| Phase 5 merge | `params_merged.json` + confidence; never infer required fields from one sample |

### J10. Troubleshooting

| Symptom | Action |
|---|---|
| Static fields never appear at runtime | Mark "pending trigger"; complete permission tree, open advanced filters, try dependent select options |
| Same path, different body shapes | Expected; record separately by `action` instead of forcing a merged schema |
| Stub responses are fake but parameters are needed | Inspect **outbound request** bodies/headers; do not infer parameters from stub responses |
| 400 mentions a nested field | Check outer `data`/`bizData`/`variables` wrappers |
| Only GraphQL operation names visible | Expand `variables` JSON; search statically for `$var: Type` |

---
