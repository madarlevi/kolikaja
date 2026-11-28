// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package menus

import (
	"testing"
	"time"
)

func TestToday(t *testing.T) {
	budapest, err := time.LoadLocation("Europe/Budapest")
	if err != nil {
		t.Fatal(err)
	}
	// 23:30 UTC on 28 November is already 29 November in Budapest.
	got := today(budapest, time.Date(2026, 11, 28, 23, 30, 0, 0, time.UTC))
	if got.Format(dateLayout) != "2026-11-29" || got.Location() != time.UTC {
		t.Fatalf("today = %v", got)
	}
}

func TestRatingWindow(t *testing.T) {
	budapest, _ := time.LoadLocation("Europe/Budapest")
	now := time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)
	day := func(s string) time.Time { d, _ := time.Parse(dateLayout, s); return d }
	one := 1
	row := func(date, status string) mealRow {
		var r mealRow
		r.MenuDate = day(date)
		r.Detail.Menu.Status = status
		return r
	}
	unlimited := dormitoryContext{Location: budapest}
	windowed := dormitoryContext{Location: budapest, RatingWindowDays: &one}
	cases := []struct {
		name string
		row  mealRow
		dc   dormitoryContext
		open bool
	}{
		{"draft", row("2026-11-28", "draft"), unlimited, false},
		{"future", row("2026-11-29", "published"), unlimited, false},
		{"today", row("2026-11-28", "published"), unlimited, true},
		{"old, no window", row("2020-01-01", "published"), unlimited, true},
		{"yesterday, 1-day window", row("2026-11-27", "published"), windowed, true},
		{"two days ago, 1-day window", row("2026-11-26", "published"), windowed, false},
	}
	for _, c := range cases {
		open, closes := ratingWindow(c.row, c.dc, now)
		if open != c.open {
			t.Errorf("%s: open = %v, want %v", c.name, open, c.open)
		}
		if c.name == "yesterday, 1-day window" {
			// Closes at the end of 28 November, Budapest time.
			want := time.Date(2026, 11, 29, 0, 0, 0, 0, budapest).UTC()
			if closes == nil || !closes.Equal(want) {
				t.Errorf("closes = %v, want %v", closes, want)
			}
		}
	}
}
