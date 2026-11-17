package agent

import (
	"context"
	"encoding/json"
	"strings"

	"antifox/db"
	actool "antifox/engine/tool"
	"antifox/knowledge"
)

// =====================================================================
// Unified asset insertion tools
// =====================================================================

// SetAssetStore wires the asset store and company store onto this ToolSet
// so the insert_assets, add_company_scope, and list_assets tools are active.
func (t *ToolSet) SetAssetStore(as *db.AssetStore, cs *db.CompanyStore) {
	t.as = as
	t.cs = cs
	if t.kb == nil && as != nil { // server-level ToolSet (nil ExplorationStore): wire KB off the asset store's DB
		t.kb = knowledge.NewService(as.DB())
	}
}

// assetInputItem is one element of the insert_assets "assets" array.
type assetInputItem struct {
	Type string `json:"type"` // root_domain|ip|subdomain|app|service|endpoint

	// ---- root_domain / subdomain ----
	Domain      string   `json:"domain"`
	ICP         string   `json:"icp"`
	RecordType  string   `json:"record_type"`
	RecordValue []string `json:"record_value"`

	// ---- ip ----
	IP           string           `json:"ip"`
	BoundDomains []string         `json:"bound_domains"`
	OpenPorts    []db.PortService `json:"open_ports"`

	// ---- app ----
	AppName     string `json:"app_name"`
	BundleID    string `json:"bundle_id"`
	Category    string `json:"category"`
	Description string `json:"description"`
	AppICP      string `json:"app_icp"`
	CompanyID   *int64 `json:"company_id"` // explicit company link (app only; others auto-attribute via scope)

	// ---- service (http) ----
	URL           string           `json:"url"`
	Technologies  []string         `json:"technologies"`
	StatusCode    *int             `json:"status_code"`
	ContentLength *int64           `json:"content_length"`
	PageTitle     string           `json:"page_title"`
	FaviconMMH3   string           `json:"favicon_mmh3"`
	Auth          []map[string]any `json:"auth"`
	ServiceName   string           `json:"service_name"`
	ServiceIP     string           `json:"service_ip"` // optional enrichment IP

	// ---- service (other) ----
	Port  int    `json:"port"`
	Proto string `json:"proto"`

	// ---- endpoint ----
	Method string           `json:"method"`
	Params []map[string]any `json:"params"`
}

// insertAssets is the unified insert_assets agent tool.
func (t *ToolSet) insertAssets() actool.CoreTool {
	return writeTool(
		"insert_assets",
		"Immediately batch newly discovered assets into the asset store. Supported types: root_domain/ip/subdomain/app/service/endpoint.\n"+
			"Asset fields:\n"+
			"• root_domain: domain (required), icp (optional registration)\n"+
			"• ip: ip (required), bound_domains ([domains]), open_ports ([{port,service}])\n"+
			"• subdomain: domain (required), record_type (A/AAAA/CNAME...), record_value, icp (optional)\n"+
			"• app: app_name (required), bundle_id (optional), category, description, app_icp, company_id (optional owner ID)\n"+
			"• service(http): url (required), technologies ([fingerprints]), status_code, content_length, page_title, favicon_mmh3, auth ([{type,username,password,...}]), service_ip\n"+
			"• service(other): service_name (required), ip or domain (at least one), port (required), auth ([...])\n"+
			"• endpoint: url (required), method (required), params ([{location,name,value,type}]), service_ip\n"+
			"auth/technologies/params are append-merged and do not overwrite existing values.\n"+
			"Returns: {results:[{index,id,type}], errors:[{index,error}]}",
		obj(map[string]any{
			"task_id": intp("Optional task ID for task_ids association"),
			"assets": map[string]any{
				"type":        "array",
				"description": "Assets; each item represents one asset record",
				"items": obj(map[string]any{
					"type": map[string]any{
						"type":        "string",
						"enum":        []string{"root_domain", "ip", "subdomain", "app", "service", "endpoint"},
						"description": "Asset type",
					},
					// root_domain / subdomain
					"domain":      str("Root domain or subdomain; required for root_domain/subdomain"),
					"icp":         str("Optional ICP registration number"),
					"record_type": str("Optional DNS record type for subdomain: A/AAAA/CNAME/MX, etc."),
					"record_value": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional DNS record values for subdomain, e.g. [\"1.2.3.4\",\"2.3.4.5\"]",
					},
					// ip
					"ip": str("IP address; required for ip assets, optional for service/endpoint IP association"),
					"bound_domains": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional domains bound to this IP",
					},
					"open_ports": map[string]any{
						"type":        "array",
						"description": "Optional open ports for IP assets",
						"items": obj(map[string]any{
							"port":    intp("Port number"),
							"service": str("Optional service name, e.g. http/ssh/mysql"),
						}, "port"),
					},
					// app
					"app_name":    str("Application name; required for app assets"),
					"bundle_id":   str("Optional bundle ID for app assets"),
					"category":    str("Optional application category"),
					"description": str("Optional application description"),
					"app_icp":     str("Optional application ICP registration"),
					"company_id":  intp("Optional owning company ID for app assets. Apps require explicit ownership because scope cannot infer it; add_company_scope returns this ID."),
					// service (http)
					"url":         str("Full URL including scheme and port; required for HTTP services, automatically sets service_type=http"),
					"status_code": intp("Optional HTTP response status, e.g. 200/301/403/404"),
					"content_length": map[string]any{
						"type":        "integer",
						"description": "Optional HTTP response body length in bytes",
					},
					"page_title":   str("Optional page <title> content"),
					"favicon_mmh3": str("Optional favicon MMH3 hash"),
					"technologies": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional technology fingerprints, e.g. [\"Nginx\",\"Vue\",\"Bootstrap\"]",
					},
					"auth": map[string]any{
						"type":        "array",
						"description": "Optional discovered authentication entries with type/username/password, append-merged",
						"items":       map[string]any{"type": "object"},
					},
					// Non-HTTP service
					"service_name": str("Service name, e.g. ssh/mysql/redis; required for non-HTTP services"),
					"port":         intp("Port number; required for non-HTTP services"),
					// endpoint
					"method": str("HTTP method: GET/POST/PUT/PATCH/DELETE, etc.; required for endpoints"),
					"params": map[string]any{
						"type":        "array",
						"description": "Optional request parameters with location(query/body/header/path)/name/value/type, append-merged",
						"items":       map[string]any{"type": "object"},
					},
				}, "type"),
			},
		}, "assets"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			if t.as == nil {
				return actool.Errorf("insert_assets is unavailable: AssetStore is not initialized"), nil
			}
			var a struct {
				Assets []assetInputItem `json:"assets"`
				TaskID int64            `json:"task_id"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return actool.Errorf("invalid input: " + err.Error()), nil
			}
			// use ToolSet.taskID as fallback
			taskID := a.TaskID
			if taskID == 0 {
				taskID = t.taskID
			}

			type result struct {
				Index int    `json:"index"`
				ID    int64  `json:"id"`
				Type  string `json:"type"`
			}
			type errEntry struct {
				Index int    `json:"index"`
				Error string `json:"error"`
			}

			var results []result
			var errs []errEntry

			for i, item := range a.Assets {
				typ := strings.TrimSpace(item.Type)
				var id int64
				var err error

				switch typ {
				case "root_domain":
					id, err = t.as.UpsertRootDomain(db.UpsertRootDomainReq{
						Domain: item.Domain,
						ICP:    item.ICP,
						TaskID: taskID,
					})

				case "ip":
					id, err = t.as.UpsertIP(db.UpsertIPReq{
						IP:           item.IP,
						BoundDomains: item.BoundDomains,
						OpenPorts:    item.OpenPorts,
						TaskID:       taskID,
					})

				case "subdomain":
					id, err = t.as.UpsertSubdomain(db.UpsertSubdomainReq{
						Domain:      item.Domain,
						RecordType:  item.RecordType,
						RecordValue: item.RecordValue,
						ICP:         item.ICP,
						TaskID:      taskID,
					})

				case "app":
					id, err = t.as.UpsertApp(db.UpsertAppReq{
						Name:        item.AppName,
						BundleID:    item.BundleID,
						Category:    item.Category,
						Description: item.Description,
						ICP:         item.AppICP,
						CompanyID:   item.CompanyID,
						TaskID:      taskID,
					})

				case "service":
					// distinguish HTTP vs other by presence of url
					if item.URL != "" {
						// agent may send "ip" or "service_ip" for the enrichment IP; accept both
						svcIP := item.ServiceIP
						if svcIP == "" {
							svcIP = item.IP
						}
						id, err = t.as.UpsertHTTPService(db.UpsertHTTPServiceReq{
							URL:           item.URL,
							Technologies:  item.Technologies,
							StatusCode:    item.StatusCode,
							ContentLength: item.ContentLength,
							PageTitle:     item.PageTitle,
							FaviconMMH3:   item.FaviconMMH3,
							Auth:          item.Auth,
							IP:            svcIP,
							TaskID:        taskID,
						})
					} else {
						id, err = t.as.UpsertOtherService(db.UpsertOtherServiceReq{
							Domain:      item.Domain,
							IP:          item.IP,
							Port:        item.Port,
							ServiceName: item.ServiceName,
							Auth:        item.Auth,
							TaskID:      taskID,
						})
					}

				case "endpoint":
					id, err = t.as.UpsertEndpoint(db.UpsertEndpointReq{
						URL:    item.URL,
						Method: item.Method,
						Params: item.Params,
						IP:     item.ServiceIP,
						TaskID: taskID,
					})

				default:
					errs = append(errs, errEntry{Index: i, Error: "unknown type: " + typ})
					continue
				}

				if err != nil {
					errs = append(errs, errEntry{Index: i, Error: err.Error()})
					continue
				}
				results = append(results, result{Index: i, ID: id, Type: typ})
				t.writes.Assets++
				t.anchorOwner(id)
			}

			return jsonResult(map[string]any{
				"results": results,
				"errors":  errs,
			})
		},
	)
}

// addCompanyScope writes to company_scope table and triggers asset attribution.
func (t *ToolSet) addCompanyScope() actool.CoreTool {
	return writeTool(
		"add_company_scope",
		"Add root domains, IPs, or CIDR ranges to a company's asset scope. Matching existing and future assets are automatically assigned.\n"+
			"Company names are unique: create a missing company or merge scope into an existing one.\n"+
			"One scope entry per line: root domain, IP, or CIDR (e.g. 1.2.3.0/24).\n"+
			"Always provide reason with ownership evidence such as WHOIS, certificates, or ASN.\n"+
			"Guardrails reject bare TLDs and overly broad ranges (IPv4<=/16, IPv6<=/32). Invalid lines are skipped and returned in errors.",
		obj(map[string]any{
			"company": str("Unique company name; create if missing, otherwise reuse"),
			"scope":   str("Asset scope, one root domain/IP/CIDR per line"),
			"reason":  str("Required ownership evidence/source"),
			"logo":    str("Optional company logo URL; applies only when creating a company"),
		}, "company", "scope"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			if t.cs == nil {
				return actool.Errorf("add_company_scope is unavailable: CompanyStore is not initialized"), nil
			}
			var a struct {
				Company string `json:"company"`
				Scope   string `json:"scope"`
				Reason  string `json:"reason"`
				Logo    string `json:"logo"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return actool.Errorf(err.Error()), nil
			}
			if strings.TrimSpace(a.Company) == "" {
				return actool.Errorf("company must not be empty"), nil
			}
			companyID, _, err := t.cs.UpsertCompany(a.Company, a.Logo)
			if err != nil {
				return actool.Errorf("Failed to create/get company: " + err.Error()), nil
			}
			lines := splitLines(a.Scope)
			added, skipped, invalid, errMsgs := t.cs.AddScope(companyID, lines, a.Reason)
			out := map[string]any{
				"company_id": companyID,
				"added":      added,
				"skipped":    skipped,
				"invalid":    invalid,
			}
			if len(errMsgs) > 0 {
				out["errors"] = errMsgs
			}
			return jsonResult(out)
		},
	)
}

// listAssets lets an agent query the asset table.
func (t *ToolSet) listAssets() actool.CoreTool {
	return readTool(
		"list_assets",
		"Query assets using DSL expressions or direct id/ids lookup, with pagination.\n"+
			"DSL: field=value fuzzy (ILIKE), field==value exact, field!=value exclusion, port>80 numeric comparison, bare terms for full-text fuzzy search.\n"+
			"Logic: AND / OR keywords, AND takes precedence; parentheses group expressions.\n"+
			"Filter asset types with the separate type parameter, not a DSL type field.\n"+
			"Without id/ids, dsl must be nonempty; unfiltered queries of all assets are prohibited.\n"+
			"Fields:\n"+
			"  domain      Domain (root/subdomain/service domain)\n"+
			"  root_domain Root domain (subdomain/service assets only)\n"+
			"  ip          IPv4/IPv6 address\n"+
			"  url         Full URL (service/endpoint)\n"+
			"  page_title  Page title (HTTP service)\n"+
			"  icp         ICP registration (root domain)\n"+
			"  service_name Non-HTTP service name\n"+
			"  app_name    Application name (app assets)\n"+
			"  method      HTTP method (endpoints, e.g. GET/POST)\n"+
			"  service_type Service type: http|other\n"+
			"  record_type DNS record type (subdomains, e.g. A/CNAME)\n"+
			"  technology  Technology fingerprint array (= fuzzy, == exact)\n"+
			"  port        Integer port, supports > >= < <=\n"+
			"  status_code Integer HTTP status, supports > >= < <=\n"+
			"  company_id  Owning company ID (integer)\n"+
			"  task_id     Source task ID (integer)\n"+
			"Examples: status_code>=400 AND technology=shiro\n"+
			"      method==POST AND url=/api/admin\n"+
			"      domain=example.com OR domain=example.org\n"+
			"      (port==80 OR port==443) AND technology=nginx",
		obj(map[string]any{
			"dsl":    str(`DSL query; see tool description. Required without id/ids. Examples: status_code>=400 technology=shiro / method==POST url=/api/admin`),
			"type":   str("Asset type: root_domain|ip|subdomain|app|service|endpoint. Separate from and combinable with dsl; type alone is insufficient without dsl."),
			"id":     intp("Optional direct asset ID lookup; mutually exclusive with dsl/type"),
			"ids":    map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Optional direct lookup of multiple asset IDs; mutually exclusive with dsl/type"},
			"limit":  intp("Optional result limit; default 10"),
			"offset": intp("Optional pagination offset; default 0"),
		}),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			if t.as == nil {
				return actool.Errorf("list_assets is unavailable: AssetStore is not initialized"), nil
			}
			var a struct {
				DSL    string  `json:"dsl"`
				Type   string  `json:"type"`
				ID     int64   `json:"id"`
				IDs    []int64 `json:"ids"`
				Limit  int     `json:"limit"`
				Offset int     `json:"offset"`
			}
			_ = json.Unmarshal(in, &a)
			if a.Limit <= 0 {
				a.Limit = 10
			}

			var assets []*db.Asset
			var err error
			switch {
			case a.ID > 0:
				assets, err = t.as.GetByIDs([]int64{a.ID})
			case len(a.IDs) > 0:
				assets, err = t.as.GetByIDs(a.IDs)
			case a.DSL != "":
				assets, err = t.as.QueryDSL(a.DSL, a.Type, a.Limit, a.Offset)
			default:
				return actool.Errorf("dsl must not be empty without id/ids: supply query conditions; unfiltered asset queries are not allowed"), nil
			}
			if err != nil {
				return actool.Errorf("DSL error: " + err.Error()), nil
			}
			return jsonResult(map[string]any{
				"count":  len(assets),
				"assets": assets,
			})
		},
	)
}

// listCompanies enumerates companies with their scope and asset count.
func (t *ToolSet) listCompanies() actool.CoreTool {
	return readTool(
		"list_companies",
		"List companies, their asset scopes, and assigned asset counts. "+
			"Use company_id for app ownership in insert_assets and filtering in list_assets. "+
			"Optional search filters company names case-insensitively; empty returns all.",
		obj(map[string]any{
			"search": str("Optional case-insensitive company-name filter; empty returns all"),
		}),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			if t.cs == nil {
				return actool.Errorf("list_companies is unavailable: CompanyStore is not initialized"), nil
			}
			var a struct {
				Search string `json:"search"`
			}
			_ = json.Unmarshal(in, &a)
			cos, err := t.cs.ListCompanies()
			if err != nil {
				return actool.Errorf("Failed to query companies: " + err.Error()), nil
			}
			q := strings.ToLower(strings.TrimSpace(a.Search))
			type companyOut struct {
				ID         int64    `json:"id"`
				Name       string   `json:"name"`
				AssetCount int      `json:"asset_count"`
				Scope      []string `json:"scope"`
			}
			out := make([]companyOut, 0, len(cos))
			for _, c := range cos {
				if q != "" && !strings.Contains(strings.ToLower(c.Name), q) {
					continue
				}
				scope := make([]string, 0, len(c.Scope))
				for _, r := range c.Scope {
					scope = append(scope, r.Raw)
				}
				out = append(out, companyOut{ID: c.ID, Name: c.Name, AssetCount: c.AssetCount, Scope: scope})
			}
			return jsonResult(map[string]any{"count": len(out), "companies": out})
		},
	)
}

// splitLines splits a multi-line string into non-empty trimmed lines.
func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// WorkerTools returns the tool set for a work agent.
func (t *ToolSet) WorkerTools() []actool.CoreTool {
	return []actool.CoreTool{
		t.listFindings(), t.listFacts(), t.nodeDetail(),
		t.addFinding(), t.recordFact(),
		// cross-work retrieval
		t.searchAllWorkerTraces(), t.listWorkerTraces(), t.getWorkerTrace(),
		// asset management (handlers guard nil store internally)
		t.insertAssets(), t.addCompanyScope(), t.listAssets(), t.listCompanies(),
		// knowledge base (global + this task); nil store guarded internally
		t.searchKnowledge(), t.listKnowledgeCategories(),
	}
}

// MainAgentTools returns the human-interface tool set.
func (t *ToolSet) MainAgentTools() []actool.CoreTool {
	return []actool.CoreTool{
		t.graphOverview(), t.listFindings(), t.listFacts(), t.nodeDetail(),
		t.getWorkerOutput(), t.getWorkerTrace(), t.searchAllWorkerTraces(), t.addHint(), t.addIntent(),
		// asset management (handlers guard nil store internally)
		t.insertAssets(), t.addCompanyScope(), t.listAssets(),
		// knowledge base (global + this task)
		t.searchKnowledge(), t.listKnowledgeCategories(),
	}
}

// AllDomainTools returns the union of all domain tools across all agent types,
// deduped by name (mainagent order wins). Used by the server to build a registry
// for injecting domain tools into agents (Auto, custom) that don't own a per-task
// ToolSet. The caller provides real stores; tools are callable at taskID=0 scope.
func (t *ToolSet) AllDomainTools() []actool.CoreTool {
	seen := map[string]bool{}
	var out []actool.CoreTool
	all := append(append(t.MainAgentTools(), t.PlannerTools()...), t.WorkerTools()...)
	for _, tool := range all {
		if !seen[tool.Name()] {
			seen[tool.Name()] = true
			out = append(out, tool)
		}
	}
	return out
}
