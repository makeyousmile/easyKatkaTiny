package app

import (
	"bytes"
	"testing"
)

func TestFormatNumberK(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "—"},
		{-10, "—"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0k"},
		{18400, "18.4k"},
		{25050, "25.1k"},
	}

	for _, tc := range tests {
		got := formatNumberK(tc.input)
		if got != tc.want {
			t.Errorf("formatNumberK(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSortEntries(t *testing.T) {
	entries := []ratingEntryData{
		{Name: "PlayerB", Winrate: 50.0, Games: 20},
		{Name: "PlayerA", Winrate: 65.0, Games: 20},
		{Name: "PlayerC", Winrate: 50.0, Games: 30}, // Same winrate, more games -> higher
		{Name: "PlayerD", Winrate: 40.0, Games: 10},
	}

	sortEntries(entries)

	if entries[0].Name != "PlayerA" {
		t.Errorf("expected PlayerA at rank 1, got %s", entries[0].Name)
	}
	if entries[1].Name != "PlayerC" {
		t.Errorf("expected PlayerC at rank 2 (more games), got %s", entries[1].Name)
	}
	if entries[2].Name != "PlayerB" {
		t.Errorf("expected PlayerB at rank 3, got %s", entries[2].Name)
	}
	if entries[3].Name != "PlayerD" {
		t.Errorf("expected PlayerD at rank 4, got %s", entries[3].Name)
	}
}

func TestRenderMatchCard_PNGOutput(t *testing.T) {
	in := matchCardInput{
		PlayerName:  "TestPlayer",
		HeroID:      31,
		HeroName:    "Lich",
		HeroShort:   "lich",
		IsWin:       true,
		Kills:       4,
		Deaths:      6,
		Assists:     22,
		Duration:    2185,
		HeroDamage:  18400,
		NetWorth:    14200,
		Items:       []string{"Glimmer Cape", "Tranquil Boots", "Force Staff"},
		StreakCount: 3,
		StreakIsWin: true,
		MatchID:     7829104812,
		StartTime:   1775899140,
	}

	pngData, err := renderMatchCard(in)
	if err != nil {
		t.Fatalf("renderMatchCard failed: %v", err)
	}
	if len(pngData) == 0 {
		t.Fatalf("renderMatchCard produced empty data")
	}

	// Verify PNG magic bytes
	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.HasPrefix(pngData, pngHeader) {
		t.Errorf("rendered data is not a valid PNG")
	}
}

func TestRenderRatingCard_PNGOutput(t *testing.T) {
	inputs := []ratingCardInput{
		{Rank: 1, Name: "SpacePilot3000", Winrate: 60.0, Games: 50},
		{Rank: 2, Name: "rdd", Winrate: 58.0, Games: 50},
		{Rank: 3, Name: "Верш", Winrate: 48.0, Games: 50},
		{Rank: 4, Name: "Dota Брат", Winrate: 44.0, Games: 50},
	}

	pngData, err := renderRatingCard(inputs)
	if err != nil {
		t.Fatalf("renderRatingCard failed: %v", err)
	}
	if len(pngData) == 0 {
		t.Fatalf("renderRatingCard produced empty data")
	}

	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.HasPrefix(pngData, pngHeader) {
		t.Errorf("rendered data is not a valid PNG")
	}
}

func TestBuildMatchCard_StatsEnrichment(t *testing.T) {
	m := recentMatch{
		MatchID:    0,
		HeroID:     1,
		Kills:      10,
		Deaths:     2,
		Assists:    8,
		Duration:   2400, // 40 mins
		PlayerSlot: 0,
		RadiantWin: true,
		HeroDamage: 32500,
		GoldPerMin: 650,
	}

	heroes := map[int]string{1: "Anti-Mage"}
	data, _, err := buildMatchCard(0, m, "TestCarry", "", heroes, true, 2)
	if err != nil {
		t.Fatalf("buildMatchCard failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("buildMatchCard returned empty bytes")
	}
}
