// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package security

import (
	"encoding/json"
	"net/url"
	"sort"
	"strings"

	"github.com/madarlevi/kolikaja/apps/api/internal/contract"
)

// Endpoint is one contract operation with its injection points.
type Endpoint struct {
	Operation contract.Operation
	// PathParams, QueryParams and BodyFields name the inputs that receive payloads.
	PathParams  []string
	QueryParams []string
	BodyFields  []string
	doc         *contract.Document
}

// Unsafe reports whether the operation changes state.
func (e Endpoint) Unsafe() bool { return e.Operation.Method != "GET" }

// InjectionPoints counts the inputs that receive payloads.
func (e Endpoint) InjectionPoints() int {
	return len(e.PathParams) + len(e.QueryParams) + len(e.BodyFields)
}

// Endpoints derives the endpoint list from the contract. Every operation is included,
// also those without injection points (they are still probed for safe error handling).
func Endpoints(doc *contract.Document) []Endpoint {
	out := make([]Endpoint, 0, len(doc.Operations))
	for _, op := range doc.Operations {
		e := Endpoint{Operation: op, doc: doc}
		for _, p := range op.Parameters {
			switch p.In {
			case "path":
				e.PathParams = append(e.PathParams, p.Name)
			case "query":
				e.QueryParams = append(e.QueryParams, p.Name)
			}
		}
		if op.BodySchema != nil {
			e.BodyFields = stringFields(doc, op.BodySchema, "", 0)
			sort.Strings(e.BodyFields)
		}
		out = append(out, e)
	}
	return out
}

// stringFields returns dotted names of string-typed properties (one nesting level deep
// is enough for the contract's request bodies).
func stringFields(doc *contract.Document, s *contract.Schema, prefix string, depth int) []string {
	s = doc.Resolve(s)
	if s == nil || depth > 2 {
		return nil
	}
	var fields []string
	for name, prop := range s.Properties {
		prop = firstConcrete(doc, prop)
		if prop == nil {
			continue
		}
		full := prefix + name
		switch {
		case prop.Type.Has("string"):
			fields = append(fields, full)
		case prop.Type.Has("object"):
			fields = append(fields, stringFields(doc, prop, full+".", depth+1)...)
		}
	}
	return fields
}

// firstConcrete resolves references and returns the first non-null branch of oneOf/allOf.
func firstConcrete(doc *contract.Document, s *contract.Schema) *contract.Schema {
	s = doc.Resolve(s)
	if s == nil {
		return nil
	}
	for _, list := range [][]*contract.Schema{s.OneOf, s.AnyOf, s.AllOf} {
		for _, branch := range list {
			b := doc.Resolve(branch)
			if b != nil && (len(b.Type) != 1 || !b.Type.Has("null")) {
				return firstConcrete(doc, b)
			}
		}
	}
	return s
}

// URL builds the request URL with the payload in every path and query parameter.
// The base URL is the API root without /api/v1 (health probes live at the root).
func (e Endpoint) URL(base, payload string) string {
	path := e.Operation.Path
	for _, name := range e.PathParams {
		path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(payload))
	}
	prefix := "/api/v1"
	if path == "/healthz" || path == "/readyz" {
		prefix = ""
	}
	u := strings.TrimRight(base, "/") + prefix + path
	if len(e.QueryParams) > 0 {
		q := url.Values{}
		for _, name := range e.QueryParams {
			q.Set(name, payload)
		}
		u += "?" + q.Encode()
	}
	return u
}

// Body builds a JSON request body: the payload in every string field, plausible values
// elsewhere, so that validation reaches the string fields.
func (e Endpoint) Body(payload string) ([]byte, bool) {
	if e.Operation.BodySchema == nil {
		return nil, false
	}
	v := e.sample(e.Operation.BodySchema, payload, 0)
	data, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	return data, true
}

func (e Endpoint) sample(s *contract.Schema, payload string, depth int) any {
	s = firstConcrete(e.doc, s)
	if s == nil || depth > 3 {
		return nil
	}
	switch {
	case s.Type.Has("string"):
		return payload
	case s.Type.Has("integer"), s.Type.Has("number"):
		if s.Minimum != nil {
			return *s.Minimum
		}
		return 1
	case s.Type.Has("boolean"):
		return false
	case s.Type.Has("array"):
		return []any{e.sample(s.Items, payload, depth+1)}
	case s.Type.Has("object") || s.Properties != nil:
		obj := map[string]any{}
		for name, prop := range s.Properties {
			obj[name] = e.sample(prop, payload, depth+1)
		}
		return obj
	case len(s.Enum) > 0:
		return s.Enum[0]
	default:
		return payload
	}
}
