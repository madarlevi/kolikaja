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

// extraChecks lists object-level and role-relation checks that come on top of the tenant
// check (docs/ARCHITECTURE.md §5, §10; contract descriptions). Same-dormitory "other
// member's resource" cases must be part of the operation's TestAuthz_ test.
type extraCheck struct{ operationID, note string }

var extraCheckList = []extraCheck{
	{"updateComment", "owner check: author only"},
	{"deleteComment", "owner check: author, or moderator"},
	{"deletePhoto", "owner check: uploader, or moderator"},
	{"completePhotoUpload", "owner check: uploader only"},
	{"putMyRating", "own record of the caller"},
	{"deleteMyRating", "own record of the caller"},
	{"putMyRecommendation", "own record of the caller"},
	{"deleteMyRecommendation", "own record of the caller"},
	{"revokeMySession", "owner check: session of the caller"},
	{"markNotificationRead", "owner check: notification of the caller"},
	{"completeAvatarUpload", "owner check: upload of the caller"},
	{"getAvatarImage", "viewer shares an active membership with the owner"},
	{"updateMember", "target role below the caller; never self"},
	{"removeMember", "target role below the caller; never self"},
	{"createSanction", "target is a current or former member, role below the caller; cited report about the target"},
	{"revokeSanction", "sanction of this dormitory"},
	{"listReports", "subject's role below the caller; never reports about the caller"},
	{"getReport", "subject's role below the caller; never reports about the caller"},
	{"resolveReport", "subject's role below the caller; never reports about the caller"},
	{"escalateReport", "subject's role below the caller; never reports about the caller"},
	{"adminAssignDormitoryAdmin", "target is an active, unbanned member of the dormitory; never the caller; staff notified"},
	{"adminListEscalatedReports", "never reports about the caller"},
	{"adminResolveReport", "escalated reports only; never reports about the caller"},
	{"adminUpdateUser", "last global admin protected; all global admins notified"},
	{"adminCreatePasswordResetToken", "never self or a current or recent (30 days) global admin; owner and global admins notified"},
	{"adminCreateSanction", "never self or a current or recent (30 days) global admin"},
	{"adminDeleteDormitory", "audit-logged"},
	{"adminRestoreDormitory", "audit-logged"},
}

var extraChecks = func() map[string]string {
	m := make(map[string]string, len(extraCheckList))
	for _, c := range extraCheckList {
		m[c.operationID] = c.note
	}
	return m
}()

// BuildRows classifies every operation. existingTests is the set of test function names
// found in the authorization test directory. An operation whose path matches no known
// scoping rule is an error, so new path prefixes cannot slip through unclassified.
func BuildRows(doc *contract.Document, existingTests map[string]bool) ([]Row, error) {
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
		var err error
		row.DormitoryScoped, row.TenantCheck, err = scope(op)
		if err != nil {
			return nil, err
		}
		if extra, ok := extraChecks[op.ID]; ok {
			row.TenantCheck += "; " + extra
		}
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
	return rows, nil
}

// UnknownExtraChecks returns the operationIds in extraChecks that the contract does not
// declare (a renamed or removed operation would otherwise lose its object-level note).
func UnknownExtraChecks(doc *contract.Document) []string {
	known := map[string]bool{}
	for _, op := range doc.Operations {
		known[op.ID] = true
	}
	var unknown []string
	for id := range extraChecks {
		if !known[id] {
			unknown = append(unknown, id)
		}
	}
	sort.Strings(unknown)
	return unknown
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

func scope(op contract.Operation) (scoped, check string, err error) {
	p := op.Path
	switch {
	case strings.HasPrefix(p, "/dormitories/{dormitoryId}"):
		return scopePath, checkBoth, nil
	case strings.HasPrefix(p, "/invitations/"):
		return scopeInvitation, checkBoth, nil
	case strings.HasPrefix(p, "/avatars/"):
		return scopeShared, checkBoth, nil
	// Lists that contain dormitory data are filtered by the caller's active memberships.
	case p == "/me/ratings" || p == "/me/stats" || strings.HasPrefix(p, "/me/notifications"):
		return scopeMine, checkMineRepo, nil
	case p == "/me" || strings.HasPrefix(p, "/me/"):
		return scopeOwn, checkOwner, nil
	case strings.HasPrefix(p, "/admin/"):
		return scopeInstance, checkAdmin, nil
	case p == "/dormitories" && op.Method == "GET":
		return scopeDirectory, checkPublic, nil
	case p == "/dormitories" && op.Method == "POST":
		return scopeNo, checkPolicy, nil
	case p == "/auth/logout":
		return scopeOwn, checkOwner, nil
	case p == "/meta" || p == "/healthz" || p == "/readyz" || strings.HasPrefix(p, "/auth/"):
		return scopeNo, checkNone, nil
	default:
		return "", "", fmt.Errorf("authzmatrix: no scoping rule for %s %s (%s); add one to scope() in matrix.go", op.Method, op.Path, op.ID)
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
	b.WriteString("  membership and role, and every repository query also filters by `dormitory_id`. Object-level checks\n")
	b.WriteString("  (owner of the resource, role of the target, report subject) follow after a semicolon.\n")
	b.WriteString("- **Tests**: test function in `apps/api/tests/authz/`. Every dormitory-scoped operation needs a\n")
	b.WriteString("  cross-dormitory test, and operations with an object-level check also an \"other member's resource\"\n")
	b.WriteString("  case (template in `.claude/skills/authz-tenant-isolation/SKILL.md`). `planned` tests are written in\n")
	b.WriteString("  phase 2; the column switches automatically once the function exists, and the phase 2 security gate runs\n")
	b.WriteString("  the check with `AUTHZ_REQUIRE_TESTS=1`, which fails while any scoped row is still planned.\n\n")
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
// With requireTests, every dormitory-scoped operation must have an implemented test
// (used by the phase 2 security gate, see docs/SECURITY-PLAN.md).
func Check(doc *contract.Document, rows []Row, committed string, requireTests bool) []string {
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
		if requireTests && r.NeedsCrossTenantTest && strings.HasPrefix(r.Tests, "planned") {
			problems = append(problems, fmt.Sprintf("dormitory-scoped operation %s still has only a planned test (%s)", r.OperationID, TestName(r.OperationID)))
		}
	}
	if len(problems) == 0 && committed != Render(rows) {
		problems = append(problems, "docs/api/authz-matrix.md is out of date; run scripts/check-authz-matrix.sh --write and commit the result")
	}
	return problems
}
