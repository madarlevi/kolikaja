// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package authzmatrix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madarlevi/kolikaja/apps/api/internal/contract"
)

const miniSpec = `
openapi: 3.1.0
security:
  - cookieAuth: []
  - bearerAuth: []
paths:
  /meta:
    get:
      operationId: getMeta
      security: []
      x-required-role: public
  /dormitories/{dormitoryId}/menus:
    get:
      operationId: listMenus
      security:
        - cookieAuth: []
        - bearerAuth: []
        - apiKeyAuth: []
      x-required-role: member
      x-api-key-scopes: [menus:read]
    post:
      operationId: createMenu
      security:
        - cookieAuth: []
          csrfToken: []
        - bearerAuth: []
      x-required-role: moderator
  /dormitories/{dormitoryId}/comments/{commentId}:
    patch:
      operationId: updateComment
      x-required-role: member
  /me/notifications:
    get:
      operationId: listNotifications
      x-required-role: authenticated
  /admin/users:
    get:
      operationId: adminListUsers
      x-required-role: global_admin
`

func parse(t *testing.T, spec string) *contract.Document {
	t.Helper()
	doc, err := contract.Parse([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func build(t *testing.T, doc *contract.Document, existing map[string]bool) []Row {
	t.Helper()
	rows, err := BuildRows(doc, existing)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestBuildRowsClassifiesOperations(t *testing.T) {
	rows := build(t, parse(t, miniSpec), map[string]bool{"TestAuthz_ListMenus": true})
	byID := map[string]Row{}
	for _, r := range rows {
		byID[r.OperationID] = r
	}
	tests := []struct {
		id, auth, global, dorm, scoped, tests string
	}{
		{"getMeta", "public", "–", "–", scopeNo, "n/a (public)"},
		{"listMenus", "session cookie; bearer; API key (menus:read)", "–", "member", scopePath, "`TestAuthz_ListMenus`"},
		{"createMenu", "session cookie + CSRF; bearer", "–", "moderator", scopePath, "planned: `TestAuthz_CreateMenu`"},
		{"listNotifications", "session cookie; bearer", "–", "–", scopeMine, "planned: `TestAuthz_ListNotifications`"},
		{"adminListUsers", "session cookie; bearer", "global_admin", "–", scopeInstance, "planned: `TestAuthz_AdminListUsers`"},
	}
	for _, tt := range tests {
		r, ok := byID[tt.id]
		if !ok {
			t.Fatalf("row %s missing", tt.id)
		}
		if r.Authentication != tt.auth || r.GlobalRole != tt.global || r.DormitoryRole != tt.dorm ||
			r.DormitoryScoped != tt.scoped || r.Tests != tt.tests {
			t.Errorf("%s: got %+v", tt.id, r)
		}
	}
	if !strings.Contains(byID["updateComment"].TenantCheck, "owner check: author only") {
		t.Errorf("updateComment should carry its owner check, got %q", byID["updateComment"].TenantCheck)
	}
}

func TestBuildRowsRejectsUnknownPathPrefix(t *testing.T) {
	doc := parse(t, `
paths:
  /reports-export:
    get:
      operationId: exportEverything
      x-required-role: authenticated
`)
	if _, err := BuildRows(doc, nil); err == nil || !strings.Contains(err.Error(), "no scoping rule") {
		t.Fatalf("expected an unclassified-path error, got %v", err)
	}
}

func TestCheckDetectsMissingAndUnknownRows(t *testing.T) {
	doc := parse(t, miniSpec)
	rows := build(t, doc, nil)
	good := Render(rows)
	if problems := Check(doc, rows, good, false); len(problems) != 0 {
		t.Fatalf("rendered matrix should pass, got %v", problems)
	}

	missing := strings.Replace(good, "| `createMenu` |", "| removed |", 1)
	if problems := Check(doc, rows, missing, false); !containsText(problems, "createMenu") {
		t.Errorf("missing row not reported: %v", problems)
	}

	unknown := good + "| `deleteEverything` | `DELETE /x` | public | – | – | no | n/a | n/a |\n"
	if problems := Check(doc, rows, unknown, false); !containsText(problems, "deleteEverything") {
		t.Errorf("unknown row not reported: %v", problems)
	}

	stale := strings.Replace(good, "moderator", "member", 1)
	if problems := Check(doc, rows, stale, false); !containsText(problems, "out of date") {
		t.Errorf("stale matrix not reported: %v", problems)
	}
}

func TestCheckRequireTests(t *testing.T) {
	doc := parse(t, miniSpec)
	all := map[string]bool{"TestAuthz_ListMenus": true, "TestAuthz_CreateMenu": true, "TestAuthz_UpdateComment": true, "TestAuthz_ListNotifications": true}
	partial := build(t, doc, map[string]bool{"TestAuthz_ListMenus": true})
	if problems := Check(doc, partial, Render(partial), true); !containsText(problems, "createMenu still has only a planned test") {
		t.Errorf("planned test not reported in require mode: %v", problems)
	}
	complete := build(t, doc, all)
	if problems := Check(doc, complete, Render(complete), true); len(problems) != 0 {
		t.Errorf("complete matrix should pass in require mode: %v", problems)
	}
}

func TestFindTests(t *testing.T) {
	dir := t.TempDir()
	src := "package authz\n\nfunc TestAuthz_ListMenus(t *testing.T) {}\nfunc helper() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "menus_test.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	found, err := FindTests(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !found["TestAuthz_ListMenus"] || len(found) != 1 {
		t.Errorf("found = %v", found)
	}
	none, err := FindTests(filepath.Join(dir, "missing"))
	if err != nil || len(none) != 0 {
		t.Errorf("missing dir: %v %v", none, err)
	}
}

// TestRepositoryMatrixIsCurrent runs the same check as scripts/check-authz-matrix.sh,
// so `go test ./...` also catches a stale matrix.
func TestRepositoryMatrixIsCurrent(t *testing.T) {
	specPath, err := contract.FindSpec(".")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := contract.Load(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if unknown := UnknownExtraChecks(doc); len(unknown) > 0 {
		t.Fatalf("object-level checks for unknown operations: %v", unknown)
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(specPath)))
	existing, err := FindTests(filepath.Join(root, "apps", "api", "tests", "authz"))
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(filepath.Clean(filepath.Join(root, "docs", "api", "authz-matrix.md")))
	if err != nil {
		t.Fatal(err)
	}
	rows := build(t, doc, existing)
	if problems := Check(doc, rows, string(committed), false); len(problems) != 0 {
		t.Fatalf("authorization matrix problems:\n%s", strings.Join(problems, "\n"))
	}
	if got := len(ParseOperationIDs(string(committed))); got != len(doc.Operations) {
		t.Fatalf("matrix has %d rows, contract has %d operations", got, len(doc.Operations))
	}
}

func containsText(list []string, text string) bool {
	for _, s := range list {
		if strings.Contains(s, text) {
			return true
		}
	}
	return false
}
