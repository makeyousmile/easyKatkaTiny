package app

import (
	"strings"
	"testing"
)

func TestEvaluateRuin_CleanPlayer(t *testing.T) {
	in := RuinEvaluationInput{
		Duration:     2100, // 35 mins
		Kills:        8,
		Deaths:       5,
		Assists:      12,
		GPM:          520,
		XPM:          650,
		HeroDamage:   24000,
		NetWorth:     21000,
		LeaverStatus: 0,
		Pings:        15,
		Items:        []string{"Power Treads", "Black King Bar", "Manta Style", "Butterfly"},
		IsWin:        true,
	}

	analysis := EvaluateRuin(in)
	if analysis.IsRuiner {
		t.Errorf("expected clean player, but flagged as ruiner (score: %d)", analysis.Score)
	}
	if analysis.IsSuspect {
		t.Errorf("expected clean player, but flagged as suspect (score: %d)", analysis.Score)
	}
	if analysis.Score > 20 {
		t.Errorf("expected low score, got %d", analysis.Score)
	}
	if analysis.Verdict != "Чисто" {
		t.Errorf("expected verdict 'Чисто', got %q", analysis.Verdict)
	}
}

func TestEvaluateRuin_IntentionalFeeder(t *testing.T) {
	in := RuinEvaluationInput{
		Duration:     1500, // 25 mins
		Kills:        0,
		Deaths:       17,
		Assists:      1,
		GPM:          180,
		XPM:          210,
		HeroDamage:   1100,
		NetWorth:     3000,
		LeaverStatus: 0,
		Items:        []string{"Boots of Speed"},
		IsWin:        false,
	}

	analysis := EvaluateRuin(in)
	if !analysis.IsRuiner {
		t.Errorf("expected ruiner, got score %d (verdict: %s)", analysis.Score, analysis.Verdict)
	}
	if analysis.Score < 65 {
		t.Errorf("expected score >= 65, got %d", analysis.Score)
	}
	if len(analysis.Reasons) == 0 {
		t.Errorf("expected ruiner reasons, got none")
	}

	alert := formatRuinAlert(analysis)
	if !strings.Contains(alert, "РУИНЕР") {
		t.Errorf("expected alert to contain РУИНЕР, got %s", alert)
	}
}

func TestEvaluateRuin_ShadowAmuletAFK(t *testing.T) {
	in := RuinEvaluationInput{
		Duration:     1800, // 30 mins
		Kills:        1,
		Deaths:       4,
		Assists:      2,
		GPM:          140,
		XPM:          180,
		HeroDamage:   900,
		NetWorth:     2500,
		LeaverStatus: 0,
		Items:        []string{"Boots of Speed", "Shadow Amulet"},
		IsWin:        false,
	}

	analysis := EvaluateRuin(in)
	if !analysis.IsRuiner {
		t.Errorf("expected ruiner for Shadow Amulet AFK, got score %d", analysis.Score)
	}

	foundReason := false
	for _, r := range analysis.Reasons {
		if strings.Contains(r, "Shadow Amulet") {
			foundReason = true
			break
		}
	}
	if !foundReason {
		t.Errorf("expected 'Shadow Amulet' reason, got %v", analysis.Reasons)
	}
}

func TestEvaluateRuin_TrollBoots(t *testing.T) {
	in := RuinEvaluationInput{
		Duration:     1600,
		Kills:        2,
		Deaths:       11,
		Assists:      3,
		GPM:          260,
		XPM:          290,
		HeroDamage:   4500,
		NetWorth:     5500,
		LeaverStatus: 0,
		Items:        []string{"Boots of Speed", "Phase Boots", "Power Treads", "Tranquil Boots"},
		IsWin:        false,
	}

	analysis := EvaluateRuin(in)
	if !analysis.IsRuiner {
		t.Errorf("expected ruiner for 4 boots troll build, got score %d", analysis.Score)
	}
}

func TestEvaluateRuin_AbandonLeaver(t *testing.T) {
	in := RuinEvaluationInput{
		Duration:     1200,
		Kills:        1,
		Deaths:       8,
		Assists:      2,
		GPM:          200,
		XPM:          220,
		HeroDamage:   2000,
		LeaverStatus: 2, // Abandoned
		Items:        []string{"Boots of Speed"},
		IsWin:        false,
	}

	analysis := EvaluateRuin(in)
	if !analysis.IsRuiner {
		t.Errorf("expected ruiner for abandon (leaver_status=2), got score %d", analysis.Score)
	}
}

func TestEvaluateRuin_SoldItems(t *testing.T) {
	in := RuinEvaluationInput{
		Duration:     2200, // 36 mins
		Kills:        3,
		Deaths:       9,
		Assists:      5,
		GPM:          320,
		XPM:          380,
		HeroDamage:   7000,
		NetWorth:     1500, // destroyed items
		LeaverStatus: 0,
		Items:        []string{"Iron Branch"},
		IsWin:        false,
	}

	analysis := EvaluateRuin(in)
	if !analysis.IsRuiner {
		t.Errorf("expected ruiner for sold items, got score %d", analysis.Score)
	}
}

func TestRenderMatchCard_WithRuinerBadge(t *testing.T) {
	in := matchCardInput{
		PlayerName:  "TiltFeeder",
		HeroID:      1,
		HeroName:    "Anti-Mage",
		HeroShort:   "antimage",
		IsWin:       false,
		Kills:       0,
		Deaths:      16,
		Assists:     1,
		Duration:    1400,
		HeroDamage:  1200,
		NetWorth:    2400,
		Items:       []string{"Boots of Speed"},
		MatchID:     987654321,
		RuinAnalysis: RuinAnalysis{
			Score:     85,
			IsRuiner:  true,
			Verdict:   "Руинер",
			BadgeText: "РУИНЕР",
			Reasons:   []string{"Умышленный фид", "Критически низкий урон"},
		},
	}

	data, err := renderMatchCard(in)
	if err != nil {
		t.Fatalf("renderMatchCard with ruiner badge failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("renderMatchCard produced empty data")
	}
}

func TestEvaluateMatchRuiners_WithAllyAndEnemyRuiners(t *testing.T) {
	heroes := map[int]string{1: "Anti-Mage", 14: "Pudge", 8: "Juggernaut"}
	itemNames := map[int]string{63: "Power Treads", 29: "Boots of Speed"}

	details := matchDetails{
		MatchID:    12345678,
		Duration:   1800, // 30 mins
		RadiantWin: true,
		Players: []matchDetailsPlayer{
			{
				AccountID:   1001,
				PersonaName: "CarryPlayer",
				HeroID:      8, // Juggernaut
				PlayerSlot:  0, // Radiant
				Kills:       12,
				Deaths:      2,
				Assists:     8,
				GPM:         650,
				XPM:         700,
				HeroDamage:  28000,
				NetWorth:    22000,
				Item0:       63,
			},
			{
				AccountID:   1002,
				PersonaName: "FeederAlly",
				HeroID:      14, // Pudge
				PlayerSlot:  1,  // Radiant (Ally)
				Kills:       0,
				Deaths:      16,
				Assists:     1,
				GPM:         150,
				XPM:         180,
				HeroDamage:  1100,
				NetWorth:    1800,
				Item0:       29,
			},
			{
				AccountID:    2001,
				PersonaName:  "AbandonEnemy",
				HeroID:       1,   // Anti-Mage
				PlayerSlot:   128, // Dire (Enemy)
				Kills:        1,
				Deaths:       5,
				Assists:      0,
				GPM:          240,
				XPM:          260,
				HeroDamage:   3000,
				NetWorth:     4000,
				LeaverStatus: 2, // Abandoned
			},
		},
	}

	ruiners := EvaluateMatchRuiners(details, 1001, 0, heroes, itemNames)
	if len(ruiners) != 2 {
		t.Fatalf("expected 2 ruiners, got %d", len(ruiners))
	}

	// First should be FeederAlly (Ally)
	if ruiners[0].PlayerName != "FeederAlly" || !ruiners[0].IsAlly {
		t.Errorf("expected ruiners[0] to be FeederAlly (ally), got %+v", ruiners[0])
	}
	if !ruiners[0].Analysis.IsRuiner {
		t.Errorf("expected FeederAlly to be flagged as Ruiner, score: %d", ruiners[0].Analysis.Score)
	}

	// Second should be AbandonEnemy (Enemy)
	if ruiners[1].PlayerName != "AbandonEnemy" || ruiners[1].IsAlly {
		t.Errorf("expected ruiners[1] to be AbandonEnemy (enemy), got %+v", ruiners[1])
	}
	if !ruiners[1].Analysis.IsRuiner {
		t.Errorf("expected AbandonEnemy to be flagged as Ruiner, score: %d", ruiners[1].Analysis.Score)
	}

	alert := formatRuinAlert(RuinAnalysis{MatchRuiners: ruiners})
	if !strings.Contains(alert, "В матче обнаружены руинеры (2)") {
		t.Errorf("expected multi-ruiner header in alert, got:\n%s", alert)
	}
	if !strings.Contains(alert, "[Союзник]") {
		t.Errorf("expected [Союзник] in alert, got:\n%s", alert)
	}
	if !strings.Contains(alert, "[Противник]") {
		t.Errorf("expected [Противник] in alert, got:\n%s", alert)
	}
	if !strings.Contains(alert, "Pudge (FeederAlly)") {
		t.Errorf("expected FeederAlly mentioned in alert, got:\n%s", alert)
	}
}

func TestEvaluateMatchRuiners_CleanMatch(t *testing.T) {
	heroes := map[int]string{1: "Anti-Mage"}
	details := matchDetails{
		MatchID:    12345678,
		Duration:   1800,
		RadiantWin: true,
		Players: []matchDetailsPlayer{
			{
				AccountID:   1001,
				PersonaName: "GoodPlayer",
				HeroID:      1,
				PlayerSlot:  0,
				Kills:       8,
				Deaths:      4,
				Assists:     10,
				GPM:         550,
				XPM:         600,
				HeroDamage:  22000,
				NetWorth:    18000,
			},
		},
	}

	ruiners := EvaluateMatchRuiners(details, 1001, 0, heroes, nil)
	if len(ruiners) != 0 {
		t.Errorf("expected 0 ruiners in clean match, got %d", len(ruiners))
	}

	alert := formatRuinAlert(RuinAnalysis{MatchRuiners: ruiners})
	if alert != "" {
		t.Errorf("expected empty alert for clean match, got %q", alert)
	}
}
