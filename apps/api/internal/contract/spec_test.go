// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package contract

import (
	"testing"
)

func loadRepoSpec(t *testing.T) *Document {
	t.Helper()
	path, err := FindSpec(".")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestLoadRepositoryContract(t *testing.T) {
	doc := loadRepoSpec(t)
	if len(doc.Operations) == 0 {
		t.Fatal("no operations found")
	}
	byID := map[string]Operation{}
	for _, op := range doc.Operations {
		byID[op.ID] = op
	}

	meta, ok := byID["getInstanceMeta"]
	if !ok || meta.Method != "GET" || meta.Path != "/meta" || !meta.IsPublic() {
		t.Errorf("getInstanceMeta = %+v, want public GET /meta", meta)
	}

	me, ok := byID["getCurrentUser"]
	if !ok || me.IsPublic() || !me.HasScheme("cookieAuth") || !me.HasScheme("bearerAuth") {
		t.Errorf("getCurrentUser should inherit the global cookie and bearer security, got %+v", me.Security)
	}

	rate, ok := byID["putMyRating"]
	if !ok {
		t.Fatal("putMyRating missing")
	}
	if rate.RequiredRole != "member" {
		t.Errorf("putMyRating role = %q, want member", rate.RequiredRole)
	}
	if got := rate.PathParams(); len(got) != 2 || got[0] != "dormitoryId" || got[1] != "mealId" {
		t.Errorf("putMyRating path params = %v", got)
	}
	if rate.BodySchema == nil || rate.BodySchema.Properties["value"] == nil {
		t.Fatalf("putMyRating body schema not resolved: %+v", rate.BodySchema)
	}
	if v := doc.Resolve(rate.BodySchema.Properties["value"]); v == nil || !v.Type.Has("integer") {
		t.Errorf("rating value should resolve to an integer schema, got %+v", v)
	}
	for _, p := range rate.Parameters {
		if p.Name == "" || p.In != "path" || p.Schema == nil {
			t.Errorf("unresolved parameter %+v", p)
		}
	}

	menus, ok := byID["listMenus"]
	if !ok || !menus.HasScheme("apiKeyAuth") || len(menus.APIKeyScopes) == 0 {
		t.Errorf("listMenus should accept scoped API keys, got %+v", menus)
	}
}

func TestParseRejectsDuplicateOperationIDs(t *testing.T) {
	spec := []byte(`
openapi: 3.1.0
paths:
  /a:
    get:
      operationId: same
  /b:
    get:
      operationId: same
`)
	if _, err := Parse(spec); err == nil {
		t.Fatal("expected an error for duplicate operationId")
	}
}

func TestTypeListAcceptsScalarAndSequence(t *testing.T) {
	doc, err := Parse([]byte(`
paths: {}
components:
  schemas:
    A:
      type: string
    B:
      type: [string, 'null']
`))
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Schemas["A"].Type.Has("string") || !doc.Schemas["B"].Type.Has("null") {
		t.Errorf("unexpected types: %v %v", doc.Schemas["A"].Type, doc.Schemas["B"].Type)
	}
}
