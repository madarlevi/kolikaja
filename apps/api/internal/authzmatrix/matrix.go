// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package authzmatrix renders and checks docs/api/authz-matrix.md, the authorization
// matrix with one row per operation of the OpenAPI contract.
//
// The matrix is generated, never edited by hand: authentication and roles come from the
// contract (security requirements, x-required-role, x-api-key-scopes), tenant scoping
// follows the rules in docs/ARCHITECTURE.md section 5, and the test column is filled
// from the test functions that exist in apps/api/tests/authz.
package authzmatrix

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/madarlevi/kolikaja/apps/api/internal/contract"
)

// Row is one line of the matrix.
type Row struct {
	OperationID     string
	MethodPath      string
	Authentication  string
	GlobalRole      string
	DormitoryRole   string
	DormitoryScoped string
	TenantCheck     string
	Tests           string
	// NeedsCrossTenantTest is true for every dormitory-scoped operation.
	NeedsCrossTenantTest bool
}

const (
	scopePath       = "yes (path)"
	scopeInvitation = "yes (derived: invitation code)"
	scopeShared     = "yes (derived: shared membership)"
	scopeMine       = "yes (derived: caller's memberships)"
	scopeNo         = "no"
	scopeOwn        = "no (caller's own data)"
	scopeInstance   = "no (instance administration)"
	scopeDirectory  = "no (public directory)"

	checkBoth     = "both: service membership/role check + repository query by dormitory_id"
	checkOwner    = "repository query (owner = caller)"
	checkMineRepo = "repository query (caller's active memberships)"
	checkAdmin    = "service (global_admin)"
	checkPolicy   = "service (instance policy)"
	checkPublic   = "repository query (public dormitories only)"
	checkNone     = "n/a"
)

// TestName is the name of the authorization test expected for an operation.
func TestName(operationID string) string {
	if operationID == "" {
		return "TestAuthz_"
	}
	return "TestAuthz_" + strings.ToUpper(operationID[:1]) + operationID[1:]
}

// BuildRows classifies every operation. existingTests is the set of test function names
// found in the authorization test directory.
func BuildRows(doc *contract.Document, existingTests map[string]bool) []Row {
	rows := make([]Row, 0, len(doc.Operations))
	for _, op := range doc.Operations {
		row := Row{
			OperationID:    op.ID,
			MethodPath:     op.Method + " " + op.Path,
			Authentication: authentication(op),
			GlobalRole:     "–",
			DormitoryRole:  "–",
		}
		switch op.RequiredRole {
		case "global_admin":
			row.GlobalRole = "global_admin"
		case "member", "moderator", "admin":
			row.DormitoryRole = op.RequiredRole
		}
		row.DormitoryScoped, row.TenantCheck = scope(op)
		row.NeedsCrossTenantTest = strings.HasPrefix(row.DormitoryScoped, "yes")

		name := TestName(op.ID)
		switch {
		case existingTests[name]:
			row.Tests = "`" + name + "`"
		case op.IsPublic() && !row.NeedsCrossTenantTest:
			row.Tests = "n/a (public)"
		default:
			row.Tests = "planned: `" + name + "`"
		}
		rows = append(rows, row)
	}
	return rows
}

func authentication(op contract.Operation) string {
	if op.IsPublic() {
		return "public"
	}
	var parts []string
	for _, req := range op.Security {
		_, csrf := req["csrfToken"]
		switch {
		case has(req, "cookieAuth") && csrf:
			parts = append(parts, "session cookie + CSRF")
		case has(req, "cookieAuth"):
			parts = append(parts, "session cookie")
		case has(req, "refreshCookie") && csrf:
			parts = append(parts, "refresh cookie + CSRF")
		case has(req, "bearerAuth"):
			parts = append(parts, "bearer")
		case has(req, "apiKeyAuth"):
			parts = append(parts, "API key ("+strings.Join(op.APIKeyScopes, ", ")+")")
		default:
			keys := make([]string, 0, len(req))
			for k := range req {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			parts = append(parts, strings.Join(keys, " + "))
		}
	}
	return strings.Join(parts, "; ")
}

func has(req contract.SecurityRequirement, scheme string) bool {
	_, ok := req[scheme]
	return ok
}

func scope(op contract.Operation) (scoped, check string) {
	p := op.Path
	switch {
	case strings.HasPrefix(p, "/dormitories/{dormitoryId}"):
		return scopePath, checkBoth
	case strings.HasPrefix(p, "/invitations/"):
		return scopeInvitation, checkBoth
	case strings.HasPrefix(p, "/avatars/"):
		return scopeShared, checkBoth
	case p == "/me/ratings" || p == "/me/stats":
		return scopeMine, checkMineRepo
	case p == "/me" || strings.HasPrefix(p, "/me/"):
		return scopeOwn, checkOwner
	case strings.HasPrefix(p, "/admin/"):
		return scopeInstance, checkAdmin
	case p == "/dormitories" && op.Method == "GET":
		return scopeDirectory, checkPublic
	case p == "/dormitories" && op.Method == "POST":
		return scopeNo, checkPolicy
	case strings.HasPrefix(p, "/auth/logout"):
		return scopeOwn, checkOwner
	default:
		return scopeNo, checkNone
	}
}

// Render produces the Markdown document.
func Render(rows []Row) string {
	var b strings.Builder
	scoped, planned, implemented := 0, 0, 0
	for _, r := range rows {
		if r.NeedsCrossTenantTest {
			scoped++
		}
		switch {
		case strings.HasPrefix(r.Tests, "planned"):
			planned++
		case strings.HasPrefix(r.Tests, "`"):
			implemented++
		}
	}
	b.WriteString("# Authorization matrix\n\n")
	b.WriteString("<!-- Generated by apps/api/internal/authzmatrix from docs/api/openapi.yaml. Do not edit by hand. -->\n\n")
	b.WriteString("Generated from `docs/api/openapi.yaml` by `scripts/check-authz-matrix.sh --write`; CI runs\n")
	b.WriteString("`scripts/check-authz-matrix.sh` and fails when an operation has no row, a row has no operation,\n")
	b.WriteString("or the file is out of date. To change a row, change the contract (security, `x-required-role`,\n")
	b.WriteString("`x-api-key-scopes`) or the scoping rules in `apps/api/internal/authzmatrix`, then regenerate.\n\n")
	fmt.Fprintf(&b, "Operations: **%d** · dormitory-scoped: **%d** · authorization tests implemented: **%d** · planned: **%d**\n\n",
		len(rows), scoped, implemented, planned)
	b.WriteString("Columns\n")
	b.WriteString("- **Authentication**: accepted credentials. `+ CSRF` means the `X-CSRF-Token` header is required with the cookie.\n")
	b.WriteString("- **Global role** / **Dormitory role**: minimum role from `x-required-role`; `–` means none beyond authentication.\n")
	b.WriteString("  Roles always come from the server-side membership, never from the request.\n")
	b.WriteString("- **Dormitory scoped**: `yes (path)` for `/dormitories/{dormitoryId}/…`; `derived` when the dormitory\n")
	b.WriteString("  comes from the resource (invitation code, shared membership, caller's memberships).\n")
	b.WriteString("- **Tenant check**: where isolation is enforced (docs/ARCHITECTURE.md §5.3). `both` = the service checks\n")
	b.WriteString("  membership and role, and every repository query also filters by `dormitory_id`.\n")
	b.WriteString("- **Tests**: test function in `apps/api/tests/authz/`. Every dormitory-scoped operation needs a\n")
	b.WriteString("  cross-dormitory test (template in `.claude/skills/authz-tenant-isolation/SKILL.md`); `planned` tests\n")
	b.WriteString("  are written in phase 2 and the column switches automatically once the function exists.\n\n")
	b.WriteString("| operationId | Method and path | Authentication | Global role | Dormitory role | Dormitory scoped | Tenant check | Tests (apps/api/tests/authz) |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s | %s | %s | %s | %s |\n",
			r.OperationID, r.MethodPath, r.Authentication, r.GlobalRole, r.DormitoryRole,
			r.DormitoryScoped, r.TenantCheck, r.Tests)
	}
	return b.String()
}

var rowPattern = regexp.MustCompile("^\\| `([A-Za-z0-9_]+)` \\|")

// ParseOperationIDs returns the operationIds of the rows in a rendered matrix, in order.
func ParseOperationIDs(markdown string) []string {
	var ids []string
	for _, line := range strings.Split(markdown, "\n") {
		if m := rowPattern.FindStringSubmatch(line); m != nil {
			ids = append(ids, m[1])
		}
	}
	return ids
}

var testFuncPattern = regexp.MustCompile(`(?m)^func (TestAuthz_[A-Za-z0-9_]+)\(`)

// FindTests returns the TestAuthz_* function names declared in Go files under dir.
// A missing directory yields an empty set.
func FindTests(dir string) (map[string]bool, error) {
	found := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return found, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Clean(filepath.Join(dir, e.Name())))
		if err != nil {
			return nil, err
		}
		for _, m := range testFuncPattern.FindAllStringSubmatch(string(data), -1) {
			found[m[1]] = true
		}
	}
	return found, nil
}

// Check compares the committed matrix with the contract and returns every problem found.
func Check(doc *contract.Document, rows []Row, committed string) []string {
	var problems []string
	inSpec := map[string]bool{}
	for _, op := range doc.Operations {
		inSpec[op.ID] = true
	}
	inMatrix := map[string]int{}
	for _, id := range ParseOperationIDs(committed) {
		inMatrix[id]++
	}
	for _, op := range doc.Operations {
		if inMatrix[op.ID] == 0 {
			problems = append(problems, fmt.Sprintf("operation %s (%s %s) has no row in the matrix", op.ID, op.Method, op.Path))
		}
	}
	ids := make([]string, 0, len(inMatrix))
	for id := range inMatrix {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !inSpec[id] {
			problems = append(problems, fmt.Sprintf("row %s refers to an operation that does not exist in the contract", id))
		}
		if inMatrix[id] > 1 {
			problems = append(problems, fmt.Sprintf("operation %s has %d rows", id, inMatrix[id]))
		}
	}
	for _, r := range rows {
		if r.NeedsCrossTenantTest && !strings.Contains(r.Tests, "TestAuthz_") {
			problems = append(problems, fmt.Sprintf("dormitory-scoped operation %s has no cross-dormitory test (planned or implemented)", r.OperationID))
		}
	}
	if len(problems) == 0 && committed != Render(rows) {
		problems = append(problems, "docs/api/authz-matrix.md is out of date; run scripts/check-authz-matrix.sh --write and commit the result")
	}
	return problems
}
