package session

import (
	"encoding/json"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
)

var (
	// Go regexp has no lookahead; use capturing groups like (/|$) instead of (?=/|$).
	reDigitsPath = regexp.MustCompile(`/\d+(/|$)`)
	reUUIDPath   = regexp.MustCompile(
		`/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}(/|$)`)
	reDatePath = regexp.MustCompile(`/\d{4}/\d{1,2}/\d{1,2}(/|$)`)
)

// NormalizePath templates numeric ids, UUIDs, and dates like Python normalize_path.
func NormalizePath(path string) string {
	path = reDigitsPath.ReplaceAllString(path, "/{id}$1")
	path = reUUIDPath.ReplaceAllString(path, "/{uuid}$1")
	path = reDatePath.ReplaceAllString(path, "/{yyyy}/{m}/{d}$1")
	return path
}

// ParseHarXHR reads HAR and returns unique XHR endpoints (Python parse_har_xhr).
func ParseHarXHR(harPath string) ([]Endpoint, error) {
	raw, err := os.ReadFile(harPath)
	if err != nil {
		return nil, err
	}
	var har struct {
		Log struct {
			Entries []map[string]any `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(raw, &har); err != nil {
		return nil, err
	}

	type row struct {
		host, pathTemplate, path, url string
		hasQuery                      bool
		methodUp                      string
	}

	var rows []row
	for _, e := range har.Log.Entries {
		rt, _ := e["_resourceType"].(string)
		if rt == "" {
			rt, _ = e["resourceType"].(string)
		}
		if rt != "xhr" {
			continue
		}
		req, _ := e["request"].(map[string]any)
		if req == nil {
			continue
		}
		rawURL, _ := req["url"].(string)
		method, _ := req["method"].(string)
		if method == "" {
			method = "GET"
		}
		method = strings.ToUpper(method)
		parsed, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		p := parsed.Path
		if p == "" {
			p = "/"
		}
		rows = append(rows, row{
			methodUp:     method,
			host:         parsed.Scheme + "://" + parsed.Host,
			path:         p,
			pathTemplate: NormalizePath(p),
			hasQuery:     parsed.RawQuery != "",
			url:          rawURL,
		})
	}

	type key struct{ m, h, t string }
	uniq := make(map[key]Endpoint)
	for _, r := range rows {
		k := key{r.methodUp, r.host, r.pathTemplate}
		if _, ok := uniq[k]; ok {
			continue
		}
		uniq[k] = Endpoint{
			Method:       r.methodUp,
			Host:         r.host,
			Path:         r.path,
			PathTemplate: r.pathTemplate,
			HasQuery:     r.hasQuery,
			URL:          r.url,
		}
	}
	out := make([]Endpoint, 0, len(uniq))
	for _, ep := range uniq {
		out = append(out, ep)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Host != out[j].Host {
			return out[i].Host < out[j].Host
		}
		if out[i].PathTemplate != out[j].PathTemplate {
			return out[i].PathTemplate < out[j].PathTemplate
		}
		return out[i].Method < out[j].Method
	})
	return out, nil
}
