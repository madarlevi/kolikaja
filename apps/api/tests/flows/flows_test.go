// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package flows tests API behavior end to end through the real router and PostgreSQL:
// authentication and session rules (docs/ARCHITECTURE.md §4), validation, error envelope,
// security headers, menus, ratings and statistics. Skipped unless KOLIKAJA_TEST_DATABASE_URL
// is set.
package flows

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/madarlevi/kolikaja/apps/api/internal/auth"
	"github.com/madarlevi/kolikaja/apps/api/tests/testenv"
)

var env *testenv.Env

func TestMain(m *testing.M) { testenv.Main(m, &env, nil) }

const v1 = testenv.V1

func setup(t *testing.T) {
	t.Helper()
	testenv.Require(t, env)
}

func expect(t *testing.T, r testenv.Response, status int, code string) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("status %d, want %d; body: %s", r.Status, status, r.Body)
	}
	if code != "" && r.ErrorCode() != code {
		t.Fatalf("error code %q, want %q; body: %s", r.ErrorCode(), code, r.Body)
	}
}

func TestRegisterValidation(t *testing.T) {
	setup(t)
	u := env.NewUser(t)
	reg := func(body any, opts ...testenv.RequestOption) testenv.Response {
		return env.Do(t, http.MethodPost, v1+"/auth/register", "", body, opts...)
	}
	r := reg(map[string]any{"email": "short@example.test", "password": "short", "display_name": "Ab"})
	expect(t, r, 422, "validation_failed")
	if !strings.Contains(string(r.Body), `"field":"password"`) || !strings.Contains(string(r.Body), "too_short") {
		t.Errorf("missing password detail: %s", r.Body)
	}
	expect(t, reg(map[string]any{"email": strings.ToUpper(u.Email), "password": "another long password", "display_name": "Ab"}), 409, "already_exists")
	expect(t, reg(map[string]any{"email": "x@example.test", "password": "another long password", "display_name": "Ab", "is_global_admin": true}), 400, "")
	expect(t, reg(`{"email":`), 400, "")
	expect(t, reg(`[]`), 400, "")
	big := `{"email":"x@example.test","password":"` + strings.Repeat("a", 300<<10) + `","display_name":"Ab"}`
	expect(t, reg(big), 413, "")
	expect(t, reg(`{}`, testenv.WithHeader("Content-Type", "text/plain")), 415, "")
	expect(t, reg(map[string]any{"email": "x@example.test", "password": "password1234", "display_name": "Ab"}), 422, "validation_failed")
}

func TestLoginErrors(t *testing.T) {
	setup(t)
	u := env.NewUser(t)
	admin := env.NewUser(t)
	login := func(email, pw string) testenv.Response {
		return env.Do(t, http.MethodPost, v1+"/auth/native/login", "", map[string]any{"email": email, "password": pw})
	}
	wrong := login(u.Email, "wrong password!!")
	unknown := login("nobody-"+uuid.NewString()+"@example.test", "wrong password!!")
	expect(t, wrong, 401, "invalid_credentials")
	expect(t, unknown, 401, "invalid_credentials")
	if string(wrong.Body[:30]) != string(unknown.Body[:30]) {
		t.Error("unknown e-mail and wrong password responses differ")
	}
	expect(t, login(strings.ToUpper(u.Email), u.Password), 200, "")

	env.Sanction(t, u, nil, "ban", admin)
	expect(t, login(u.Email, u.Password), 403, "account_banned")
	expect(t, env.Do(t, http.MethodGet, v1+"/me", u.Token, nil), 403, "account_banned")
}

func TestLoginRateLimitPerAccount(t *testing.T) {
	setup(t)
	u := env.NewUser(t)
	var last testenv.Response
	for i := 0; i < 6; i++ {
		last = env.Do(t, http.MethodPost, v1+"/auth/native/login", "", map[string]any{"email": u.Email, "password": "wrong password!!"},
			testenv.FromAddr("192.0.2.77:5555"))
	}
	expect(t, last, 429, "rate_limited")
	if last.Header.Get("Retry-After") == "" {
		t.Error("429 without Retry-After")
	}
	// Another address is not affected.
	expect(t, env.Do(t, http.MethodPost, v1+"/auth/native/login", "", map[string]any{"email": u.Email, "password": u.Password}), 200, "")
}

func TestRefreshRotationAndReuse(t *testing.T) {
	setup(t)
	u := env.NewUser(t)
	refresh := func(token string) testenv.Response {
		return env.Do(t, http.MethodPost, v1+"/auth/native/refresh", "", map[string]any{"refresh_token": token})
	}
	r := refresh(u.Refresh)
	expect(t, r, 200, "")
	var rotated testenv.Tokens
	r.Decode(t, &rotated)
	if rotated.RefreshToken == u.Refresh || rotated.RefreshToken == "" {
		t.Fatal("refresh token was not rotated")
	}
	expect(t, env.Do(t, http.MethodGet, v1+"/me", rotated.AccessToken, nil), 200, "")

	// Reusing the old token revokes the whole session.
	expect(t, refresh(u.Refresh), 401, "session_revoked")
	expect(t, refresh(rotated.RefreshToken), 401, "session_revoked")
	expect(t, env.Do(t, http.MethodGet, v1+"/me", rotated.AccessToken, nil), 401, "session_revoked")
	if n := env.Count(t, `SELECT count(*) FROM audit_logs WHERE action = 'auth.refresh_token_reused' AND target_id = $1`, rotated.Session.ID); n != 1 {
		t.Errorf("%d reuse audit rows, want 1", n)
	}
	if n := env.Count(t, `SELECT count(*) FROM user_sessions WHERE id = $1 AND revoke_reason = 'refresh_token_reuse'`, rotated.Session.ID); n != 1 {
		t.Error("session not revoked for reuse")
	}
	expect(t, refresh("x"), 401, "invalid_token")
	expect(t, refresh(strings.Repeat("A", 43)), 401, "invalid_token")
}

func TestTokenExpiry(t *testing.T) {
	setup(t)
	u := env.NewUser(t)
	env.Clock.Advance(11 * time.Minute)
	defer env.Clock.Advance(-11 * time.Minute)
	expect(t, env.Do(t, http.MethodGet, v1+"/me", u.Token, nil), 401, "unauthenticated")
	// Logout accepts the expired access token and ends the session.
	expect(t, env.Do(t, http.MethodPost, v1+"/auth/logout", u.Token, nil), 204, "")
	expect(t, env.Do(t, http.MethodPost, v1+"/auth/native/refresh", "", map[string]any{"refresh_token": u.Refresh}), 401, "session_revoked")
}

func TestRefreshTokenExpired(t *testing.T) {
	setup(t)
	u := env.NewUser(t)
	env.Clock.Advance(721 * time.Hour)
	defer env.Clock.Advance(-721 * time.Hour)
	expect(t, env.Do(t, http.MethodPost, v1+"/auth/native/refresh", "", map[string]any{"refresh_token": u.Refresh}), 401, "invalid_token")
}

func TestForgedToken(t *testing.T) {
	setup(t)
	u := env.NewUser(t)
	parts := strings.Split(u.Token, ".")
	if len(parts) != 3 {
		t.Fatal("access token is not a JWS")
	}
	// alg "none"
	none := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." + parts[1] + "."
	expect(t, env.Do(t, http.MethodGet, v1+"/me", none, nil), 401, "unauthenticated")
	// tampered payload
	tampered := parts[0] + "." + parts[1] + "x." + parts[2]
	expect(t, env.Do(t, http.MethodGet, v1+"/me", tampered, nil), 401, "unauthenticated")
}

func TestAmbiguousCredentials(t *testing.T) {
	setup(t)
	u := env.NewUser(t)
	expect(t, env.Do(t, http.MethodGet, v1+"/me", u.Token, nil, testenv.WithHeader("X-API-Key", "kk_test")), 400, "ambiguous_credentials")
}

func TestHeadersAndEnvelope(t *testing.T) {
	setup(t)
	r := env.Do(t, http.MethodGet, "/healthz", "", nil, testenv.WithHeader("X-Request-ID", "client-chosen"))
	expect(t, r, 200, "")
	for name, want := range map[string]string{
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Cache-Control":           "no-store",
		"Referrer-Policy":         "no-referrer",
	} {
		if got := r.Header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if id := r.Header.Get("X-Request-ID"); id == "" || id == "client-chosen" {
		t.Errorf("X-Request-ID = %q, want a server-generated ID", id)
	}
	expect(t, env.Do(t, http.MethodPatch, v1+"/me", "", map[string]any{}), 404, "not_found")
	nf := env.Do(t, http.MethodGet, v1+"/does-not-exist", "", nil)
	expect(t, nf, 404, "not_found")
	if !strings.Contains(string(nf.Body), `"request_id"`) {
		t.Errorf("error envelope without request_id: %s", nf.Body)
	}
	expect(t, env.Do(t, http.MethodGet, "/readyz", "", nil), 200, "")
	meta := env.Do(t, http.MethodGet, v1+"/meta", "", nil)
	expect(t, meta, 200, "")
	if !strings.Contains(string(meta.Body), `"license":"AGPL-3.0-only"`) {
		t.Errorf("meta: %s", meta.Body)
	}
}

func TestMenusRatingsAndStats(t *testing.T) {
	setup(t)
	admin, mod := env.NewUser(t), env.NewUser(t)
	members := []testenv.User{env.NewUser(t), env.NewUser(t), env.NewUser(t)}
	d := env.NewDormitory(t, admin, "private", "invite_only")
	env.AddMember(t, d, mod, "moderator")
	for _, m := range members {
		env.AddMember(t, d, m, "member")
	}
	base := v1 + "/dormitories/" + d.String()
	today := env.Today().Format("2006-01-02")

	// A moderator publishes today's lunch with two meals.
	r := env.Do(t, http.MethodPost, base+"/menus", mod.Token, map[string]any{
		"menu_date": today, "status": "published", "title": "  Heti ajánlat ",
		"meals": []map[string]any{
			{"name": "Gulyásleves", "allergens": []string{"celery"}},
			{"name": "Rántott sajt", "allergens": []string{"milk", "gluten", "eggs"}, "position": 1},
		},
	})
	expect(t, r, 201, "")
	var menu struct {
		ID     uuid.UUID `json:"id"`
		Status string    `json:"status"`
		Title  string    `json:"title"`
		Meals  []struct {
			ID   uuid.UUID `json:"id"`
			Name string    `json:"name"`
		} `json:"meals"`
	}
	r.Decode(t, &menu)
	if menu.Status != "published" || len(menu.Meals) != 2 || menu.Meals[0].Name != "Gulyásleves" || menu.Title != "Heti ajánlat" {
		t.Fatalf("created menu: %+v", menu)
	}
	expect(t, env.Do(t, http.MethodPost, base+"/menus", mod.Token, map[string]any{"menu_date": today}), 409, "conflict")
	expect(t, env.Do(t, http.MethodPost, base+"/menus", mod.Token, map[string]any{"menu_date": "2026-02-30"}), 422, "validation_failed")
	expect(t, env.Do(t, http.MethodPost, base+"/menus", mod.Token, map[string]any{"menu_date": today, "meal_period": "dinner",
		"meals": []map[string]any{{"name": "X", "allergens": []string{"nuts"}}}}), 422, "validation_failed")

	meal := menu.Meals[0].ID
	rate := func(u testenv.User, v any) testenv.Response {
		return env.Do(t, http.MethodPut, base+"/meals/"+meal.String()+"/my-rating", u.Token, map[string]any{"value": v})
	}
	expect(t, rate(members[0], 0), 422, "validation_failed")
	expect(t, rate(members[0], 6), 422, "validation_failed")
	expect(t, rate(members[0], "5"), 400, "")
	expect(t, rate(members[0], 5), 201, "")
	expect(t, rate(members[1], 4), 201, "")
	r = rate(members[2], 4)
	expect(t, r, 201, "")
	var result struct {
		Stats struct {
			RatingCount          int      `json:"rating_count"`
			RatingAverage        *float64 `json:"rating_average"`
			RatingDistribution   []int    `json:"rating_distribution"`
			TrustedRatingAverage *float64 `json:"trusted_rating_average"`
		} `json:"stats"`
	}
	r.Decode(t, &result)
	st := result.Stats
	if st.RatingCount != 3 || st.RatingAverage == nil || *st.RatingAverage != 4.33 || st.TrustedRatingAverage != nil {
		t.Errorf("stats = %+v", st)
	}
	if len(st.RatingDistribution) != 5 || st.RatingDistribution[3] != 2 || st.RatingDistribution[4] != 1 {
		t.Errorf("distribution = %v", st.RatingDistribution)
	}
	expect(t, env.Do(t, http.MethodPut, base+"/meals/"+meal.String()+"/my-recommendation", members[0].Token, map[string]any{"value": "maybe"}), 422, "validation_failed")
	recommend := func(u testenv.User, v string) testenv.Response {
		return env.Do(t, http.MethodPut, base+"/meals/"+meal.String()+"/my-recommendation", u.Token, map[string]any{"value": v})
	}
	r = recommend(members[0], "recommended")
	expect(t, r, 201, "")
	if !strings.Contains(string(r.Body), `"recommended_count":0`) {
		t.Errorf("a single recommendation must not be shown: %s", r.Body)
	}
	expect(t, recommend(members[1], "recommended"), 201, "")
	expect(t, recommend(members[2], "not_recommended"), 201, "")

	// Meal detail carries the caller's own rating and recommendation only.
	r = env.Do(t, http.MethodGet, base+"/meals/"+meal.String(), members[0].Token, nil)
	expect(t, r, 200, "")
	var detail struct {
		MyRating         *int    `json:"my_rating"`
		MyRecommendation *string `json:"my_recommendation"`
		RatingOpen       bool    `json:"rating_open"`
		Stats            struct {
			RecommendedCount int `json:"recommended_count"`
		} `json:"stats"`
	}
	r.Decode(t, &detail)
	if detail.MyRating == nil || *detail.MyRating != 5 || detail.MyRecommendation == nil || *detail.MyRecommendation != "recommended" ||
		!detail.RatingOpen || detail.Stats.RecommendedCount != 2 {
		t.Errorf("meal detail = %+v", detail)
	}
	r = env.Do(t, http.MethodGet, base+"/meals/"+meal.String(), members[1].Token, nil)
	detail.MyRecommendation = nil
	r.Decode(t, &detail)
	if detail.MyRating == nil || *detail.MyRating != 4 || detail.MyRecommendation == nil || *detail.MyRecommendation != "recommended" {
		t.Errorf("second member sees %+v", detail)
	}

	// Today and week views.
	r = env.Do(t, http.MethodGet, base+"/menus/today", members[0].Token, nil)
	expect(t, r, 200, "")
	var day struct {
		Date     string `json:"date"`
		Timezone string `json:"timezone"`
		Menus    []struct {
			Meals []struct {
				MyRating *int `json:"my_rating"`
			} `json:"meals"`
		} `json:"menus"`
	}
	r.Decode(t, &day)
	if day.Date != today || day.Timezone != "Europe/Budapest" || len(day.Menus) != 1 || len(day.Menus[0].Meals) != 2 ||
		day.Menus[0].Meals[0].MyRating == nil || *day.Menus[0].Meals[0].MyRating != 5 {
		t.Errorf("today = %+v", day)
	}
	r = env.Do(t, http.MethodGet, base+"/menus/week?date="+today, members[0].Token, nil)
	expect(t, r, 200, "")
	var week struct {
		WeekStart string `json:"week_start"`
		WeekEnd   string `json:"week_end"`
		Days      []struct {
			Date  string           `json:"date"`
			Menus []map[string]any `json:"menus"`
		} `json:"days"`
	}
	r.Decode(t, &week)
	start, _ := time.Parse("2006-01-02", week.WeekStart)
	if len(week.Days) != 7 || start.Weekday() != time.Monday || week.Days[6].Date != week.WeekEnd {
		t.Errorf("week = %+v", week)
	}
	found := false
	for _, dd := range week.Days {
		if dd.Date == today && len(dd.Menus) == 1 {
			found = true
		}
	}
	if !found {
		t.Error("today's menu missing from the week")
	}
	expect(t, env.Do(t, http.MethodGet, base+"/menus/week?date=2026-13-01", members[0].Token, nil), 400, "")

	// Deleting the rating removes it from the statistics.
	expect(t, env.Do(t, http.MethodDelete, base+"/meals/"+meal.String()+"/my-rating", members[0].Token, nil), 204, "")
	expect(t, env.Do(t, http.MethodDelete, base+"/meals/"+meal.String()+"/my-rating", members[0].Token, nil), 404, "")
	r = env.Do(t, http.MethodGet, base+"/meals/"+meal.String()+"/stats", members[1].Token, nil)
	expect(t, r, 200, "")
	if !strings.Contains(string(r.Body), `"rating_count":2`) || !strings.Contains(string(r.Body), `"rating_average":null`) ||
		!strings.Contains(string(r.Body), `"rating_distribution":[0,0,0,0,0]`) {
		t.Errorf("below 3 ratings only the count may be shown: %s", r.Body)
	}
	// Rating again after deleting creates a new rating.
	expect(t, rate(members[0], 3), 201, "")
}

func TestRatingWindow(t *testing.T) {
	setup(t)
	admin, member := env.NewUser(t), env.NewUser(t)
	d := env.NewDormitory(t, admin, "private", "invite_only")
	env.AddMember(t, d, member, "member")
	env.Exec(t, `UPDATE dormitories SET rating_window_days = 1 WHERE id = $1`, d)
	base := v1 + "/dormitories/" + d.String()
	today := env.Today()
	_, future := env.NewMenu(t, d, today.AddDate(0, 0, 1), "lunch", "published")
	_, yesterday := env.NewMenu(t, d, today.AddDate(0, 0, -1), "lunch", "published")
	_, old := env.NewMenu(t, d, today.AddDate(0, 0, -3), "lunch", "published")
	rate := func(meal uuid.UUID) testenv.Response {
		return env.Do(t, http.MethodPut, base+"/meals/"+meal.String()+"/my-rating", member.Token, map[string]any{"value": 3})
	}
	expect(t, rate(future), 409, "rating_closed")
	expect(t, rate(old), 409, "rating_closed")
	expect(t, rate(yesterday), 201, "")
	r := env.Do(t, http.MethodGet, base+"/meals/"+yesterday.String(), member.Token, nil)
	var detail struct {
		RatingOpen     bool       `json:"rating_open"`
		RatingClosesAt *time.Time `json:"rating_closes_at"`
	}
	r.Decode(t, &detail)
	if !detail.RatingOpen || detail.RatingClosesAt == nil || !detail.RatingClosesAt.After(time.Now()) {
		t.Errorf("yesterday's meal: %+v", detail)
	}
}

func TestDormitoryDirectoryAndJoin(t *testing.T) {
	setup(t)
	admin, joiner := env.NewUser(t), env.NewUser(t)
	env.Exec(t, `UPDATE users SET is_global_admin = true WHERE id = $1`, admin.ID)
	slug := "flow-" + strings.ToLower(admin.ID.String()[:8])
	r := env.Do(t, http.MethodPost, v1+"/dormitories", admin.Token, map[string]any{
		"slug": slug, "name": "Flow 100% kollégium_", "visibility": "public", "join_policy": "open", "city": "Szeged",
	})
	expect(t, r, 201, "")
	var created struct {
		ID uuid.UUID `json:"id"`
	}
	r.Decode(t, &created)
	expect(t, env.Do(t, http.MethodPost, v1+"/dormitories", admin.Token, map[string]any{
		"slug": slug, "name": "Dup", "visibility": "public", "join_policy": "open"}), 409, "already_exists")
	expect(t, env.Do(t, http.MethodPost, v1+"/dormitories", admin.Token, map[string]any{
		"slug": "Bad Slug", "name": "X", "visibility": "private", "join_policy": "open", "timezone": "Mars/Base"}), 422, "validation_failed")

	// LIKE wildcards in the query are literal.
	r = env.Do(t, http.MethodGet, v1+"/dormitories?q="+"100%25+koll", joiner.Token, nil)
	expect(t, r, 200, "")
	if !strings.Contains(string(r.Body), created.ID.String()) {
		t.Errorf("search by name with %%: %s", r.Body)
	}
	r = env.Do(t, http.MethodGet, v1+"/dormitories?q=%25", joiner.Token, nil)
	if !strings.Contains(string(r.Body), created.ID.String()) {
		t.Errorf("literal %% search should match the name containing %%: %s", r.Body)
	}
	r = env.Do(t, http.MethodGet, v1+"/dormitories?q=zzz%25zzz", joiner.Token, nil)
	if strings.Contains(string(r.Body), created.ID.String()) {
		t.Error("wildcard was not escaped")
	}
	expect(t, env.Do(t, http.MethodGet, v1+"/dormitories?cursor=@@@", joiner.Token, nil), 400, "")
	expect(t, env.Do(t, http.MethodGet, v1+"/dormitories?limit=1000", joiner.Token, nil), 400, "")

	expect(t, env.Do(t, http.MethodPost, v1+"/dormitories/"+created.ID.String()+"/join", joiner.Token, nil), 201, "")
	r = env.Do(t, http.MethodGet, v1+"/me/dormitories", joiner.Token, nil)
	if !strings.Contains(string(r.Body), created.ID.String()) {
		t.Errorf("membership missing: %s", r.Body)
	}
}

func TestSuspendedMemberIsReadOnly(t *testing.T) {
	setup(t)
	admin, member := env.NewUser(t), env.NewUser(t)
	d := env.NewDormitory(t, admin, "private", "invite_only")
	env.AddMember(t, d, member, "member")
	_, meal := env.NewMenu(t, d, env.Today(), "lunch", "published")
	env.Sanction(t, member, &d, "suspension", admin)
	base := v1 + "/dormitories/" + d.String()
	expect(t, env.Do(t, http.MethodGet, base+"/menus/today", member.Token, nil), 200, "")
	expect(t, env.Do(t, http.MethodPut, base+"/meals/"+meal.String()+"/my-rating", member.Token, map[string]any{"value": 2}), 403, "suspended")
	r := env.Do(t, http.MethodGet, base, member.Token, nil)
	if !strings.Contains(string(r.Body), `"kind":"suspension"`) {
		t.Errorf("restriction missing from my_membership: %s", r.Body)
	}
}

func TestConcurrentRefreshRevokesTheSession(t *testing.T) {
	setup(t)
	u := env.NewUser(t)
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- env.Do(t, http.MethodPost, v1+"/auth/native/refresh", "", map[string]any{"refresh_token": u.Refresh}).Status
		}()
	}
	wg.Wait()
	close(statuses)
	got := map[int]int{}
	for s := range statuses {
		got[s]++
	}
	if got[200] != 1 || got[401] != 1 {
		t.Fatalf("statuses %v, want one 200 and one 401", got)
	}
	if n := env.Count(t, `SELECT count(*) FROM user_sessions WHERE user_id = $1 AND revoke_reason = 'refresh_token_reuse'`, u.ID); n != 1 {
		t.Error("concurrent use of one refresh token did not revoke the session")
	}
}

func TestRefreshRejectsWebSessions(t *testing.T) {
	setup(t)
	u := env.NewUser(t)
	token, hash, err := auth.NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	sid := uuid.Must(uuid.NewV7())
	env.Exec(t, `INSERT INTO user_sessions (id, user_id, client_type, expires_at) VALUES ($1, $2, 'web', now() + interval '1 day')`, sid, u.ID)
	env.Exec(t, `INSERT INTO refresh_tokens (id, session_id, token_hash, expires_at) VALUES ($1, $2, $3, now() + interval '1 day')`,
		uuid.Must(uuid.NewV7()), sid, hash)
	expect(t, env.Do(t, http.MethodPost, v1+"/auth/native/refresh", "", map[string]any{"refresh_token": token}), 401, "invalid_token")
}

func TestRefreshRefusesBannedAndDeletedAccounts(t *testing.T) {
	setup(t)
	banned, admin := env.NewUser(t), env.NewUser(t)
	env.Sanction(t, banned, nil, "ban", admin)
	expect(t, env.Do(t, http.MethodPost, v1+"/auth/native/refresh", "", map[string]any{"refresh_token": banned.Refresh}), 403, "account_banned")
	if n := env.Count(t, `SELECT count(*) FROM user_sessions WHERE user_id = $1 AND revoke_reason = 'account_banned'`, banned.ID); n != 1 {
		t.Error("the banned user's session was not revoked")
	}
	deleted := env.NewUser(t)
	env.Exec(t, `UPDATE users SET email = NULL, password_hash = NULL, display_name = NULL, deleted_at = now() WHERE id = $1`, deleted.ID)
	expect(t, env.Do(t, http.MethodPost, v1+"/auth/native/refresh", "", map[string]any{"refresh_token": deleted.Refresh}), 401, "session_revoked")
}

func TestLoginFailureCeilingPerAccount(t *testing.T) {
	setup(t)
	u := env.NewUser(t)
	// Failures from many addresses count against the account; the right password is then
	// refused too until the hour is over.
	for i := 0; i < 100; i++ {
		r := env.Do(t, http.MethodPost, v1+"/auth/native/login", "", map[string]any{"email": u.Email, "password": "wrong password!!"})
		if r.Status != 401 {
			t.Fatalf("attempt %d: %d %s", i, r.Status, r.Body)
		}
	}
	expect(t, env.Do(t, http.MethodPost, v1+"/auth/native/login", "", map[string]any{"email": u.Email, "password": u.Password}), 429, "rate_limited")
	// An over-long address is rejected by validation, not stored as a limiter key.
	expect(t, env.Do(t, http.MethodPost, v1+"/auth/native/login", "", map[string]any{"email": strings.Repeat("a", 300) + "@x.test", "password": "x"}), 422, "validation_failed")
}

func TestInvisibleCharactersRejected(t *testing.T) {
	setup(t)
	r := env.Do(t, http.MethodPost, v1+"/auth/register", "", map[string]any{
		"email": "rtl-" + uuid.NewString()[:8] + "@example.test", "password": "correct horse battery staple", "display_name": "Admin\u202eevil",
	})
	expect(t, r, 422, "validation_failed")
}
