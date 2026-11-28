// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package authz holds the cross-dormitory authorization tests, one TestAuthz_<OperationId>
// per operation (template: .claude/skills/authz-tenant-isolation/SKILL.md). They run the real
// router against PostgreSQL and are skipped unless KOLIKAJA_TEST_DATABASE_URL is set.
package authz

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/madarlevi/kolikaja/apps/api/tests/testenv"
)

var env *testenv.Env

func TestMain(m *testing.M) { testenv.Main(m, &env, nil) }

const v1 = testenv.V1

// fixture: two private, invite-only dormitories A and B.
//
//	adminA, moderatorA, memberA, member2A  members of A
//	outsider                               member of B only
//	adminB                                 administrator of B
type fixture struct {
	dormA, dormB, publicOpen, publicInvite uuid.UUID
	adminA, moderatorA, memberA, member2A  testenv.User
	outsider, adminB                       testenv.User
	formerA, suspendedA                    testenv.User // left A; suspended instance-wide (member of A)
	deletedDorm                            uuid.UUID    // soft-deleted; memberA is still listed as member
	todayMealA, todayMealB, draftMealA     uuid.UUID
}

var (
	fixOnce sync.Once
	fix     fixture
	dayBack atomic.Int64
)

func setup(t *testing.T) *fixture {
	t.Helper()
	testenv.Require(t, env)
	fixOnce.Do(func() {
		f := &fix
		f.adminA, f.moderatorA, f.memberA, f.member2A = env.NewUser(t), env.NewUser(t), env.NewUser(t), env.NewUser(t)
		f.outsider, f.adminB = env.NewUser(t), env.NewUser(t)
		f.dormA = env.NewDormitory(t, f.adminA, "private", "invite_only")
		f.dormB = env.NewDormitory(t, f.adminB, "private", "invite_only")
		env.AddMember(t, f.dormA, f.moderatorA, "moderator")
		env.AddMember(t, f.dormA, f.memberA, "member")
		env.AddMember(t, f.dormA, f.member2A, "member")
		env.AddMember(t, f.dormB, f.outsider, "member")
		f.publicOpen = env.NewDormitory(t, f.adminB, "public", "open")
		f.publicInvite = env.NewDormitory(t, f.adminB, "public", "invite_only")
		today := env.Today()
		_, f.todayMealA = env.NewMenu(t, f.dormA, today, "lunch", "published")
		_, f.todayMealB = env.NewMenu(t, f.dormB, today, "lunch", "published")
		_, f.draftMealA = env.NewMenu(t, f.dormA, today, "dinner", "draft")
		f.formerA, f.suspendedA = env.NewUser(t), env.NewUser(t)
		env.AddMember(t, f.dormA, f.formerA, "moderator")
		env.Exec(t, `UPDATE dormitory_members SET left_at = now(), leave_reason = 'left' WHERE user_id = $1`, f.formerA.ID)
		env.AddMember(t, f.dormA, f.suspendedA, "moderator")
		env.Sanction(t, f.suspendedA, nil, "suspension", f.adminB)
		f.deletedDorm = env.NewDormitory(t, f.adminA, "public", "open")
		env.AddMember(t, f.deletedDorm, f.memberA, "member")
		env.NewMenu(t, f.deletedDorm, today, "lunch", "published")
		env.Exec(t, `UPDATE dormitories SET deleted_at = now(), deleted_by = $2 WHERE id = $1`, f.deletedDorm, f.adminA.ID)
	})
	if fix.dormA == uuid.Nil {
		t.Fatal("fixture setup failed")
	}
	return &fix
}

// pastMeal creates a published meal on a unique past date (rating is open).
func pastMeal(t *testing.T, dormitoryID uuid.UUID) uuid.UUID {
	t.Helper()
	day := env.Today().AddDate(0, 0, -int(dayBack.Add(1)))
	_, meal := env.NewMenu(t, dormitoryID, day, "lunch", "published")
	return meal
}

type check struct {
	name   string
	user   *testenv.User // nil: anonymous
	method string
	path   string
	body   any
	want   int
	code   string // expected error code, optional
}

func run(t *testing.T, checks ...check) []testenv.Response {
	t.Helper()
	out := make([]testenv.Response, len(checks))
	for i, c := range checks {
		token := ""
		if c.user != nil {
			token = c.user.Token
		}
		res := env.Do(t, c.method, c.path, token, c.body)
		if res.Status != c.want {
			t.Errorf("%s: %s %s = %d, want %d; body: %s", c.name, c.method, c.path, res.Status, c.want, res.Body)
		} else if c.code != "" && res.ErrorCode() != c.code {
			t.Errorf("%s: error code %q, want %q", c.name, res.ErrorCode(), c.code)
		}
		out[i] = res
	}
	return out
}

func dorm(id uuid.UUID) string { return v1 + "/dormitories/" + id.String() }

func meal(d, m uuid.UUID) string { return dorm(d) + "/meals/" + m.String() }

func TestAuthz_GetCurrentUser(t *testing.T) {
	f := setup(t)
	res := run(t,
		check{name: "anonymous", method: http.MethodGet, path: v1 + "/me", want: 401, code: "unauthenticated"},
		check{name: "garbage token", user: &testenv.User{Token: "not-a-jwt"}, method: http.MethodGet, path: v1 + "/me", want: 401},
		check{name: "forged user id in query", user: &f.memberA, method: http.MethodGet, path: v1 + "/me?user_id=" + f.adminB.ID.String(), want: 200},
	)
	var me struct {
		ID uuid.UUID `json:"id"`
	}
	res[2].Decode(t, &me)
	if me.ID != f.memberA.ID {
		t.Fatalf("GET /me returned %s, want the caller %s", me.ID, f.memberA.ID)
	}
}

func TestAuthz_ListMyDormitories(t *testing.T) {
	f := setup(t)
	res := run(t,
		check{name: "anonymous", method: http.MethodGet, path: v1 + "/me/dormitories", want: 401},
		check{name: "member of A", user: &f.memberA, method: http.MethodGet, path: v1 + "/me/dormitories", want: 200},
		check{name: "member of B", user: &f.outsider, method: http.MethodGet, path: v1 + "/me/dormitories?user_id=" + f.memberA.ID.String(), want: 200},
	)
	ids := func(r testenv.Response) map[uuid.UUID]string {
		var list struct {
			Data []struct {
				Dormitory struct {
					ID uuid.UUID `json:"id"`
				} `json:"dormitory"`
				Role string `json:"role"`
			} `json:"data"`
		}
		r.Decode(t, &list)
		out := map[uuid.UUID]string{}
		for _, m := range list.Data {
			out[m.Dormitory.ID] = m.Role
		}
		return out
	}
	if got := ids(res[1]); len(got) != 1 || got[f.dormA] != "member" {
		t.Errorf("member of A sees %v, want only A as member", got)
	}
	if got := ids(res[2]); len(got) != 1 || got[f.dormB] != "member" {
		t.Errorf("member of B sees %v, want only B (forged user_id ignored)", got)
	}
}

func TestAuthz_Logout(t *testing.T) {
	setup(t)
	a, b := env.NewUser(t), env.NewUser(t)
	run(t,
		check{name: "anonymous", method: http.MethodPost, path: v1 + "/auth/logout", want: 401},
		check{name: "logout a", user: &a, method: http.MethodPost, path: v1 + "/auth/logout", want: 204},
		check{name: "a's token is revoked", user: &a, method: http.MethodGet, path: v1 + "/me", want: 401, code: "session_revoked"},
		check{name: "b's session is untouched", user: &b, method: http.MethodGet, path: v1 + "/me", want: 200},
	)
}

func TestAuthz_SearchDormitories(t *testing.T) {
	f := setup(t)
	res := run(t,
		check{name: "anonymous", method: http.MethodGet, path: v1 + "/dormitories", want: 401},
		check{name: "member of A", user: &f.memberA, method: http.MethodGet, path: v1 + "/dormitories?limit=100", want: 200},
		check{name: "query matching a private dormitory", user: &f.memberA, method: http.MethodGet, path: v1 + "/dormitories?q=" + slugOf(t, f.dormB), want: 200},
	)
	if strings.Contains(string(res[2].Body), f.dormB.String()) || !strings.Contains(string(res[2].Body), `"data":[]`) {
		t.Errorf("search by a private dormitory's name returned it: %s", res[2].Body)
	}
	if strings.Contains(string(res[1].Body), f.deletedDorm.String()) {
		t.Error("search lists a deleted dormitory")
	}
	var page struct {
		Data []struct {
			ID         uuid.UUID `json:"id"`
			Visibility string    `json:"visibility"`
		} `json:"data"`
	}
	res[1].Decode(t, &page)
	seen := map[uuid.UUID]bool{}
	for _, d := range page.Data {
		if d.Visibility != "public" {
			t.Errorf("search returned a %s dormitory %s", d.Visibility, d.ID)
		}
		seen[d.ID] = true
	}
	if seen[f.dormA] || seen[f.dormB] {
		t.Error("search lists private dormitories")
	}
	if !seen[f.publicOpen] {
		t.Error("search does not list the public dormitory")
	}
}

func TestAuthz_CreateDormitory(t *testing.T) {
	f := setup(t)
	admin := env.NewUser(t)
	env.Exec(t, `UPDATE users SET is_global_admin = true WHERE id = $1`, admin.ID)
	body := func(slug string) map[string]any {
		return map[string]any{"slug": slug, "name": "Új kollégium", "visibility": "private", "join_policy": "invite_only"}
	}
	forged := body("forged-admin-dorm")
	forged["is_global_admin"] = true
	slug := "authz-created-" + admin.ID.String()[:8]
	res := run(t,
		check{name: "anonymous", method: http.MethodPost, path: v1 + "/dormitories", body: body("anon-dorm"), want: 401},
		check{name: "regular user", user: &f.memberA, method: http.MethodPost, path: v1 + "/dormitories", body: body("member-dorm"), want: 403, code: "forbidden"},
		check{name: "forged admin flag", user: &f.memberA, method: http.MethodPost, path: v1 + "/dormitories", body: forged, want: 400},
		check{name: "global admin", user: &admin, method: http.MethodPost, path: v1 + "/dormitories", body: body(slug), want: 201},
	)
	var d struct {
		ID           uuid.UUID `json:"id"`
		MyMembership struct {
			Role string `json:"role"`
		} `json:"my_membership"`
	}
	res[3].Decode(t, &d)
	if d.MyMembership.Role != "admin" {
		t.Errorf("creator role = %q, want admin", d.MyMembership.Role)
	}
	if n := env.Count(t, `SELECT count(*) FROM dormitories WHERE slug IN ('member-dorm', 'forged-admin-dorm', 'anon-dorm')`); n != 0 {
		t.Errorf("%d dormitories were created by unauthorized requests", n)
	}
}

func TestAuthz_GetDormitory(t *testing.T) {
	f := setup(t)
	res := run(t,
		check{name: "anonymous", method: http.MethodGet, path: dorm(f.dormA), want: 401},
		check{name: "member of B on private A", user: &f.outsider, method: http.MethodGet, path: dorm(f.dormA), want: 404, code: "not_found"},
		check{name: "admin of B on private A", user: &f.adminB, method: http.MethodGet, path: dorm(f.dormA), want: 404},
		check{name: "member of A", user: &f.memberA, method: http.MethodGet, path: dorm(f.dormA) + "?role=admin", want: 200},
		check{name: "non-member on public dormitory", user: &f.memberA, method: http.MethodGet, path: dorm(f.publicOpen), want: 200},
		check{name: "unknown id", user: &f.memberA, method: http.MethodGet, path: dorm(uuid.Must(uuid.NewV7())), want: 404},
	)
	var d struct {
		MyMembership *struct {
			Role string `json:"role"`
		} `json:"my_membership"`
	}
	res[3].Decode(t, &d)
	if d.MyMembership == nil || d.MyMembership.Role != "member" {
		t.Errorf("member of A: my_membership = %+v, want role member (forged role ignored)", d.MyMembership)
	}
	d.MyMembership = nil
	res[4].Decode(t, &d)
	if d.MyMembership != nil {
		t.Error("non-member of a public dormitory got a membership")
	}
}

func TestAuthz_JoinDormitory(t *testing.T) {
	f := setup(t)
	joiner, banned := env.NewUser(t), env.NewUser(t)
	env.Sanction(t, banned, &f.publicOpen, "ban", f.adminB)
	run(t,
		check{name: "anonymous", method: http.MethodPost, path: dorm(f.publicOpen) + "/join", want: 401},
		check{name: "private dormitory", user: &joiner, method: http.MethodPost, path: dorm(f.dormA) + "/join", want: 404},
		check{name: "public invite-only", user: &joiner, method: http.MethodPost, path: dorm(f.publicInvite) + "/join", want: 403, code: "invitation_required"},
		check{name: "banned user", user: &banned, method: http.MethodPost, path: dorm(f.publicOpen) + "/join", want: 403, code: "banned"},
		check{name: "public open", user: &joiner, method: http.MethodPost, path: dorm(f.publicOpen) + "/join", want: 201},
		check{name: "join twice", user: &joiner, method: http.MethodPost, path: dorm(f.publicOpen) + "/join", want: 409, code: "already_member"},
	)
	if n := env.Count(t, `SELECT count(*) FROM dormitory_members WHERE user_id = $1 AND role <> 'member'`, joiner.ID); n != 0 {
		t.Error("joining granted a role above member")
	}
}

func TestAuthz_ListMenus(t *testing.T) {
	f := setup(t)
	day := env.Today().Format("2006-01-02")
	q := "/menus?from=" + day + "&to=" + day
	res := run(t,
		check{name: "anonymous", method: http.MethodGet, path: dorm(f.dormA) + q, want: 401},
		check{name: "member of B on A", user: &f.outsider, method: http.MethodGet, path: dorm(f.dormA) + q, want: 404},
		check{name: "member of A", user: &f.memberA, method: http.MethodGet, path: dorm(f.dormA) + q, want: 200},
		check{name: "member asks for drafts", user: &f.memberA, method: http.MethodGet, path: dorm(f.dormA) + q + "&include_drafts=true&role=moderator", want: 403},
		check{name: "moderator asks for drafts", user: &f.moderatorA, method: http.MethodGet, path: dorm(f.dormA) + q + "&include_drafts=true", want: 200},
	)
	statuses := func(r testenv.Response) []string {
		var list struct {
			Data []struct {
				DormitoryID uuid.UUID `json:"dormitory_id"`
				Status      string    `json:"status"`
			} `json:"data"`
		}
		r.Decode(t, &list)
		var out []string
		for _, m := range list.Data {
			if m.DormitoryID != f.dormA {
				t.Errorf("menu of dormitory %s listed for A", m.DormitoryID)
			}
			out = append(out, m.Status)
		}
		return out
	}
	if got := statuses(res[2]); len(got) != 1 || got[0] != "published" {
		t.Errorf("member sees %v, want one published menu", got)
	}
	if got := statuses(res[4]); len(got) != 2 {
		t.Errorf("moderator sees %v, want published and draft", got)
	}
}

func TestAuthz_CreateMenu(t *testing.T) {
	f := setup(t)
	date := env.Today().AddDate(0, 1, 0).Format("2006-01-02")
	body := map[string]any{"menu_date": date, "meals": []map[string]any{{"name": "Lecsó"}}}
	forged := map[string]any{"menu_date": date, "dormitory_id": f.dormB.String()}
	catB := uuid.Must(uuid.NewV7())
	env.Exec(t, `INSERT INTO meal_categories (id, dormitory_id, name) VALUES ($1, $2, 'B kategória')`, catB, f.dormB)
	crossCat := map[string]any{"menu_date": env.Today().AddDate(0, 1, 1).Format("2006-01-02"),
		"meals": []map[string]any{{"name": "Lecsó", "category_id": catB.String()}}}
	run(t,
		check{name: "anonymous", method: http.MethodPost, path: dorm(f.dormA) + "/menus", body: body, want: 401},
		check{name: "member of B on A", user: &f.outsider, method: http.MethodPost, path: dorm(f.dormA) + "/menus", body: body, want: 404},
		check{name: "admin of B on A", user: &f.adminB, method: http.MethodPost, path: dorm(f.dormA) + "/menus", body: body, want: 404},
		check{name: "member below moderator", user: &f.memberA, method: http.MethodPost, path: dorm(f.dormA) + "/menus", body: body, want: 403},
		check{name: "forged dormitory_id in body", user: &f.moderatorA, method: http.MethodPost, path: dorm(f.dormA) + "/menus", body: forged, want: 400},
		check{name: "category of another dormitory", user: &f.moderatorA, method: http.MethodPost, path: dorm(f.dormA) + "/menus", body: crossCat, want: 422},
		check{name: "moderator", user: &f.moderatorA, method: http.MethodPost, path: dorm(f.dormA) + "/menus", body: body, want: 201},
	)
	if n := env.Count(t, `SELECT count(*) FROM menus WHERE dormitory_id = $1 AND menu_date >= $2`, f.dormB, env.Today().AddDate(0, 1, 0)); n != 0 {
		t.Error("a menu was created in dormitory B")
	}
}

func TestAuthz_GetTodayMenus(t *testing.T) {
	f := setup(t)
	res := run(t,
		check{name: "anonymous", method: http.MethodGet, path: dorm(f.dormA) + "/menus/today", want: 401},
		check{name: "member of B on A", user: &f.outsider, method: http.MethodGet, path: dorm(f.dormA) + "/menus/today", want: 404},
		check{name: "member of A", user: &f.memberA, method: http.MethodGet, path: dorm(f.dormA) + "/menus/today?dormitory_id=" + f.dormB.String() + "&role=admin", want: 200},
		check{name: "former member of A", user: &f.formerA, method: http.MethodGet, path: dorm(f.dormA) + "/menus/today", want: 404},
		check{name: "member of a deleted dormitory", user: &f.memberA, method: http.MethodGet, path: dorm(f.deletedDorm) + "/menus/today", want: 404},
		check{name: "member of B, forged query on A", user: &f.outsider, method: http.MethodGet, path: dorm(f.dormA) + "/menus/today?dormitory_id=" + f.dormB.String(), want: 404},
	)
	assertMeals(t, res[2], f.todayMealA, f.todayMealB, f.draftMealA)
}

func TestAuthz_GetWeekMenus(t *testing.T) {
	f := setup(t)
	path := dorm(f.dormA) + "/menus/week?date=" + env.Today().Format("2006-01-02")
	res := run(t,
		check{name: "anonymous", method: http.MethodGet, path: path, want: 401},
		check{name: "member of B on A", user: &f.outsider, method: http.MethodGet, path: path, want: 404},
		check{name: "admin of B on A", user: &f.adminB, method: http.MethodGet, path: path, want: 404},
		check{name: "member of A", user: &f.memberA, method: http.MethodGet, path: path, want: 200},
	)
	assertMeals(t, res[3], f.todayMealA, f.todayMealB, f.draftMealA)
}

// assertMeals requires want in the response body and none of the others.
func assertMeals(t *testing.T, r testenv.Response, want uuid.UUID, notWanted ...uuid.UUID) {
	t.Helper()
	body := string(r.Body)
	if !strings.Contains(body, want.String()) {
		t.Errorf("response does not contain meal %s", want)
	}
	for _, id := range notWanted {
		if strings.Contains(body, id.String()) {
			t.Errorf("response leaks meal %s", id)
		}
	}
}

func mealReadChecks(f *fixture, suffix string) []check {
	return []check{
		{name: "anonymous", method: http.MethodGet, path: meal(f.dormA, f.todayMealA) + suffix, want: 401},
		{name: "member of B on A's meal", user: &f.outsider, method: http.MethodGet, path: meal(f.dormA, f.todayMealA) + suffix, want: 404},
		{name: "member of A, B's meal under A", user: &f.memberA, method: http.MethodGet, path: meal(f.dormA, f.todayMealB) + suffix, want: 404},
		{name: "member of A, B's meal under B", user: &f.memberA, method: http.MethodGet, path: meal(f.dormB, f.todayMealB) + suffix, want: 404},
		{name: "member of A, draft meal", user: &f.memberA, method: http.MethodGet, path: meal(f.dormA, f.draftMealA) + suffix, want: 404},
		{name: "moderator of A, draft meal", user: &f.moderatorA, method: http.MethodGet, path: meal(f.dormA, f.draftMealA) + suffix, want: 200},
		{name: "member of A", user: &f.memberA, method: http.MethodGet, path: meal(f.dormA, f.todayMealA) + suffix, want: 200},
		{name: "member of A, forged query", user: &f.memberA, method: http.MethodGet, path: meal(f.dormA, f.todayMealA) + suffix + "?dormitory_id=" + f.dormB.String() + "&role=admin", want: 200},
		{name: "former member of A", user: &f.formerA, method: http.MethodGet, path: meal(f.dormA, f.todayMealA) + suffix, want: 404},
		{name: "member of B, forged query", user: &f.outsider, method: http.MethodGet, path: meal(f.dormA, f.todayMealA) + suffix + "?dormitory_id=" + f.dormB.String(), want: 404},
	}
}

func TestAuthz_GetMeal(t *testing.T) {
	f := setup(t)
	run(t, mealReadChecks(f, "")...)
}

func TestAuthz_GetMealStats(t *testing.T) {
	f := setup(t)
	run(t, mealReadChecks(f, "/stats")...)
}

func writeChecks(f *fixture, mealA, mealB uuid.UUID, method, suffix string, body any) []check {
	return []check{
		{name: "anonymous", method: method, path: meal(f.dormA, mealA) + suffix, body: body, want: 401},
		{name: "member of B on A's meal", user: &f.outsider, method: method, path: meal(f.dormA, mealA) + suffix, body: body, want: 404},
		{name: "member of A, B's meal under A", user: &f.memberA, method: method, path: meal(f.dormA, mealB) + suffix, body: body, want: 404},
		{name: "member of A, B's meal under B", user: &f.memberA, method: method, path: meal(f.dormB, mealB) + suffix, body: body, want: 404},
		{name: "member of A, draft meal", user: &f.memberA, method: method, path: meal(f.dormA, f.draftMealA) + suffix, body: body, want: 404},
	}
}

func TestAuthz_PutMyRating(t *testing.T) {
	f := setup(t)
	mealA, mealB := pastMeal(t, f.dormA), pastMeal(t, f.dormB)
	suspended := env.NewUser(t)
	env.AddMember(t, f.dormA, suspended, "member")
	env.Sanction(t, suspended, &f.dormA, "suspension", f.adminA)
	checks := writeChecks(f, mealA, mealB, http.MethodPut, "/my-rating", map[string]any{"value": 5})
	checks = append(checks,
		check{name: "suspended member", user: &suspended, method: http.MethodPut, path: meal(f.dormA, mealA) + "/my-rating", body: map[string]any{"value": 1}, want: 403, code: "suspended"},
		check{name: "forged user_id in body", user: &f.memberA, method: http.MethodPut, path: meal(f.dormA, mealA) + "/my-rating", body: map[string]any{"value": 1, "user_id": f.member2A.ID.String()}, want: 400},
		check{name: "member creates", user: &f.memberA, method: http.MethodPut, path: meal(f.dormA, mealA) + "/my-rating?dormitory_id=" + f.dormB.String(), body: map[string]any{"value": 5}, want: 201},
		check{name: "member updates", user: &f.memberA, method: http.MethodPut, path: meal(f.dormA, mealA) + "/my-rating", body: map[string]any{"value": 4}, want: 200},
		check{name: "other member rates", user: &f.member2A, method: http.MethodPut, path: meal(f.dormA, mealA) + "/my-rating", body: map[string]any{"value": 2}, want: 201},
	)
	run(t, checks...)
	if n := env.Count(t, `SELECT count(*) FROM ratings WHERE meal_id = $1 AND dormitory_id = $2 AND deleted_at IS NULL`, mealA, f.dormA); n != 2 {
		t.Errorf("%d active ratings on A's meal, want 2", n)
	}
	if n := env.Count(t, `SELECT count(*) FROM ratings WHERE meal_id = $1`, mealB); n != 0 {
		t.Errorf("%d ratings on B's meal, want 0", n)
	}
	if v := env.Count(t, `SELECT value FROM ratings WHERE meal_id = $1 AND user_id = $2 AND deleted_at IS NULL`, mealA, f.memberA.ID); v != 4 {
		t.Errorf("member's rating = %d, want 4", v)
	}
}

func TestAuthz_DeleteMyRating(t *testing.T) {
	f := setup(t)
	mealA, mealB := pastMeal(t, f.dormA), pastMeal(t, f.dormB)
	env.Exec(t, `INSERT INTO ratings (id, dormitory_id, meal_id, user_id, value) VALUES ($1, $2, $3, $4, 3)`,
		uuid.Must(uuid.NewV7()), f.dormA, mealA, f.memberA.ID)
	env.Exec(t, `INSERT INTO ratings (id, dormitory_id, meal_id, user_id, value) VALUES ($1, $2, $3, $4, 3)`,
		uuid.Must(uuid.NewV7()), f.dormB, mealB, f.outsider.ID)
	checks := writeChecks(f, mealA, mealB, http.MethodDelete, "/my-rating", nil)
	checks = append(checks,
		check{name: "other member has no rating", user: &f.member2A, method: http.MethodDelete, path: meal(f.dormA, mealA) + "/my-rating?user_id=" + f.memberA.ID.String(), want: 404},
		check{name: "owner deletes", user: &f.memberA, method: http.MethodDelete, path: meal(f.dormA, mealA) + "/my-rating", want: 204},
	)
	run(t, checks...)
	if n := env.Count(t, `SELECT count(*) FROM ratings WHERE meal_id = $1 AND deleted_at IS NULL`, mealB); n != 1 {
		t.Error("B's rating was deleted")
	}
}

func TestAuthz_PutMyRecommendation(t *testing.T) {
	f := setup(t)
	mealA, mealB := pastMeal(t, f.dormA), pastMeal(t, f.dormB)
	checks := writeChecks(f, mealA, mealB, http.MethodPut, "/my-recommendation", map[string]any{"value": "recommended"})
	checks = append(checks,
		check{name: "instance-wide suspended member", user: &f.suspendedA, method: http.MethodPut, path: meal(f.dormA, mealA) + "/my-recommendation", body: map[string]any{"value": "recommended"}, want: 403, code: "suspended"},
		check{name: "former member", user: &f.formerA, method: http.MethodPut, path: meal(f.dormA, mealA) + "/my-recommendation", body: map[string]any{"value": "recommended"}, want: 404},
		check{name: "forged user_id in body", user: &f.memberA, method: http.MethodPut, path: meal(f.dormA, mealA) + "/my-recommendation", body: map[string]any{"value": "recommended", "user_id": f.member2A.ID.String()}, want: 400},
		check{name: "other member", user: &f.member2A, method: http.MethodPut, path: meal(f.dormA, mealA) + "/my-recommendation", body: map[string]any{"value": "recommended"}, want: 201},
		check{name: "member creates", user: &f.memberA, method: http.MethodPut, path: meal(f.dormA, mealA) + "/my-recommendation?dormitory_id=" + f.dormB.String(), body: map[string]any{"value": "recommended"}, want: 201},
		check{name: "member changes", user: &f.memberA, method: http.MethodPut, path: meal(f.dormA, mealA) + "/my-recommendation", body: map[string]any{"value": "not_recommended"}, want: 200},
	)
	run(t, checks...)
	if n := env.Count(t, `SELECT count(*) FROM recommendations WHERE meal_id = $1`, mealB); n != 0 {
		t.Error("a recommendation was stored on B's meal")
	}
	if n := env.Count(t, `SELECT count(*) FROM recommendations WHERE meal_id = $1 AND user_id = $2 AND value = 'recommended'`, mealA, f.member2A.ID); n != 1 {
		t.Error("another member's recommendation was changed")
	}
}

func TestAuthz_DeleteMyRecommendation(t *testing.T) {
	f := setup(t)
	mealA, mealB := pastMeal(t, f.dormA), pastMeal(t, f.dormB)
	env.Exec(t, `INSERT INTO recommendations (id, dormitory_id, meal_id, user_id, value) VALUES ($1, $2, $3, $4, 'recommended')`,
		uuid.Must(uuid.NewV7()), f.dormA, mealA, f.memberA.ID)
	env.Exec(t, `INSERT INTO recommendations (id, dormitory_id, meal_id, user_id, value) VALUES ($1, $2, $3, $4, 'recommended')`,
		uuid.Must(uuid.NewV7()), f.dormB, mealB, f.outsider.ID)
	checks := writeChecks(f, mealA, mealB, http.MethodDelete, "/my-recommendation", nil)
	checks = append(checks,
		check{name: "other member has none", user: &f.member2A, method: http.MethodDelete, path: meal(f.dormA, mealA) + "/my-recommendation", want: 404},
		check{name: "owner deletes", user: &f.memberA, method: http.MethodDelete, path: meal(f.dormA, mealA) + "/my-recommendation", want: 204},
	)
	run(t, checks...)
	if n := env.Count(t, `SELECT count(*) FROM recommendations WHERE meal_id = $1`, mealB); n != 1 {
		t.Error("B's recommendation was deleted")
	}
}

// slugOf returns the slug of a dormitory (its name contains the slug, see testenv).
func slugOf(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var slug string
	if err := env.Pool.QueryRow(t.Context(), `SELECT slug FROM dormitories WHERE id = $1`, id).Scan(&slug); err != nil {
		t.Fatal(err)
	}
	return slug
}

func TestFormerAndSuspendedWriters(t *testing.T) {
	f := setup(t)
	mealA := pastMeal(t, f.dormA)
	date := env.Today().AddDate(0, 2, 0).Format("2006-01-02")
	run(t,
		check{name: "former moderator creates a menu", user: &f.formerA, method: http.MethodPost, path: dorm(f.dormA) + "/menus", body: map[string]any{"menu_date": date}, want: 404},
		check{name: "suspended moderator creates a menu", user: &f.suspendedA, method: http.MethodPost, path: dorm(f.dormA) + "/menus", body: map[string]any{"menu_date": date}, want: 403, code: "suspended"},
		check{name: "suspended member deletes a rating", user: &f.suspendedA, method: http.MethodDelete, path: meal(f.dormA, mealA) + "/my-rating", want: 403, code: "suspended"},
		check{name: "suspended member deletes a recommendation", user: &f.suspendedA, method: http.MethodDelete, path: meal(f.dormA, mealA) + "/my-recommendation", want: 403, code: "suspended"},
		check{name: "suspended user joins", user: &f.suspendedA, method: http.MethodPost, path: dorm(f.publicOpen) + "/join", want: 403, code: "suspended"},
		check{name: "member of a deleted dormitory reads it", user: &f.memberA, method: http.MethodGet, path: dorm(f.deletedDorm), want: 404},
	)
}
