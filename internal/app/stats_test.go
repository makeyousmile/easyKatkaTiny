package app

import (
	"strings"
	"testing"
)

func TestCalcWinrateWithCount_UsesLimit(t *testing.T) {
	matches := []recentMatch{}
	for i := 0; i < 60; i++ {
		matches = append(matches, recentMatch{RadiantWin: i%2 == 0, PlayerSlot: 0})
	}

	winrate, games := calcWinrateWithCount(matches, 50)
	if games != 50 {
		t.Fatalf("games=%d, want 50", games)
	}
	if winrate != 50 {
		t.Fatalf("winrate=%.1f, want 50.0", winrate)
	}
}

func TestCalcWinrateWithCount_LimitGreaterThanMatches(t *testing.T) {
	matches := []recentMatch{{RadiantWin: true, PlayerSlot: 0}, {RadiantWin: false, PlayerSlot: 0}, {RadiantWin: true, PlayerSlot: 0}}
	winrate, games := calcWinrateWithCount(matches, 50)
	if games != 3 {
		t.Fatalf("games=%d, want 3", games)
	}
	if winrate != (2.0*100.0/3.0) {
		t.Fatalf("winrate=%.6f, want %f", winrate, 2.0*100.0/3.0)
	}
}

func TestCalcWinrateWithCount_ZeroLimit(t *testing.T) {
	matches := []recentMatch{{RadiantWin: true, PlayerSlot: 0}}
	winrate, games := calcWinrateWithCount(matches, 0)
	if games != 0 || winrate != 0 {
		t.Fatalf("games=%d winrate=%f, want 0/0", games, winrate)
	}
}

func TestMatchWin_PlayerSlot(t *testing.T) {
	if !matchWin(recentMatch{RadiantWin: true, PlayerSlot: 0}) {
		t.Fatal("radiant player should win when RadiantWin=true")
	}
	if matchWin(recentMatch{RadiantWin: true, PlayerSlot: 128}) {
		t.Fatal("dire player should lose when RadiantWin=true")
	}
}

func TestCalcStreak(t *testing.T) {
	// Empty matches
	isWin, count := calcStreak(nil)
	if isWin != false || count != 0 {
		t.Fatalf("expected (false, 0), got (%v, %d)", isWin, count)
	}

	// 3 wins
	wins := []recentMatch{
		{RadiantWin: true, PlayerSlot: 0},
		{RadiantWin: true, PlayerSlot: 0},
		{RadiantWin: true, PlayerSlot: 0},
	}
	isWin, count = calcStreak(wins)
	if !isWin || count != 3 {
		t.Fatalf("expected (true, 3), got (%v, %d)", isWin, count)
	}

	// 4 losses
	losses := []recentMatch{
		{RadiantWin: false, PlayerSlot: 0},
		{RadiantWin: false, PlayerSlot: 0},
		{RadiantWin: false, PlayerSlot: 0},
		{RadiantWin: false, PlayerSlot: 0},
	}
	isWin, count = calcStreak(losses)
	if isWin || count != 4 {
		t.Fatalf("expected (false, 4), got (%v, %d)", isWin, count)
	}

	// Win, loss, win
	mixed := []recentMatch{
		{RadiantWin: true, PlayerSlot: 0},
		{RadiantWin: false, PlayerSlot: 0},
		{RadiantWin: true, PlayerSlot: 0},
	}
	isWin, count = calcStreak(mixed)
	if !isWin || count != 1 {
		t.Fatalf("expected (true, 1), got (%v, %d)", isWin, count)
	}
}

func TestPluralize(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "победа"},
		{2, "победы"},
		{3, "победы"},
		{4, "победы"},
		{5, "побед"},
		{11, "побед"},
		{12, "побед"},
		{14, "побед"},
		{21, "победа"},
		{22, "победы"},
		{25, "побед"},
	}
	for _, tc := range cases {
		got := pluralize(tc.n, "победа", "победы", "побед")
		if got != tc.want {
			t.Errorf("pluralize(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestFormatStreakBadge(t *testing.T) {
	// Less than 3 should return empty string
	if got := formatStreakBadge(true, 2, false); got != "" {
		t.Fatalf("expected empty for count 2, got %q", got)
	}
	if got := formatStreakBadge(false, 1, false); got != "" {
		t.Fatalf("expected empty for count 1, got %q", got)
	}

	// 3 wins HTML and plain
	if got := formatStreakBadge(true, 3, false); !strings.Contains(got, "3 победы подряд") || !strings.Contains(got, "🔥") {
		t.Fatalf("unexpected badge plain: %q", got)
	}
	if got := formatStreakBadge(true, 3, true); !strings.Contains(got, "<b>") || !strings.Contains(got, "3 победы подряд") {
		t.Fatalf("unexpected badge html: %q", got)
	}

	// 4 wins
	if got := formatStreakBadge(true, 4, true); !strings.Contains(got, "ОГНЕННЫЙ ВИНСТРИК") {
		t.Fatalf("unexpected badge 4 wins: %q", got)
	}

	// 5 wins
	if got := formatStreakBadge(true, 5, true); !strings.Contains(got, "НЕОСТАНОВИМ") || !strings.Contains(got, "5 побед подряд") {
		t.Fatalf("unexpected badge 5 wins: %q", got)
	}

	// 3 losses
	if got := formatStreakBadge(false, 3, false); !strings.Contains(got, "💀") || !strings.Contains(got, "3 поражения подряд") {
		t.Fatalf("unexpected badge 3 losses: %q", got)
	}

	// 4 losses
	if got := formatStreakBadge(false, 4, true); !strings.Contains(got, "ЛУЗСТРИК") || !strings.Contains(got, "4 поражения подряд") {
		t.Fatalf("unexpected badge 4 losses: %q", got)
	}

	// 5 losses
	if got := formatStreakBadge(false, 5, true); !strings.Contains(got, "ТИЛЬТ") || !strings.Contains(got, "5 поражений подряд") {
		t.Fatalf("unexpected badge 5 losses: %q", got)
	}
}

func TestFormatStartupStreaksMessage(t *testing.T) {
	// Empty streaks
	if got := formatStartupStreaksMessage(nil, false); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}

	// Single streak
	single := []playerStreak{
		{Name: "Dendi", IsWin: true, Streak: 3},
	}
	htmlSingle := formatStartupStreaksMessage(single, true)
	if !strings.Contains(htmlSingle, "Активная серия при запуске") || !strings.Contains(htmlSingle, "Dendi") || !strings.Contains(htmlSingle, "3 победы подряд") {
		t.Fatalf("unexpected single html: %q", htmlSingle)
	}
	plainSingle := formatStartupStreaksMessage(single, false)
	if strings.Contains(plainSingle, "<b>") || !strings.Contains(plainSingle, "Dendi") {
		t.Fatalf("unexpected single plain: %q", plainSingle)
	}

	// Multiple streaks
	multi := []playerStreak{
		{Name: "Dendi", IsWin: true, Streak: 4},
		{Name: "Puppey", IsWin: false, Streak: 3},
	}
	htmlMulti := formatStartupStreaksMessage(multi, true)
	if !strings.Contains(htmlMulti, "Активные серии при запуске") || !strings.Contains(htmlMulti, "Dendi") || !strings.Contains(htmlMulti, "Puppey") {
		t.Fatalf("unexpected multi html: %q", htmlMulti)
	}
}


