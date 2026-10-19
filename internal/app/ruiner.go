package app

import (
	"fmt"
	"sort"
	"strings"
)

// MatchPlayerRuin represents a ruiner or suspect found in a match.
type MatchPlayerRuin struct {
	AccountID  int64
	PlayerName string
	HeroID     int
	HeroName   string
	PlayerSlot int
	IsTracked  bool
	IsAlly     bool
	Analysis   RuinAnalysis
}

// RuinAnalysis represents the verdict and scoring details of a match analysis.
type RuinAnalysis struct {
	Score        int               // 0 to 100
	IsRuiner     bool              // Score >= 65
	IsSuspect    bool              // Score >= 45 && Score < 65
	Verdict      string            // "Чисто", "Подозрение на руин", "Руинер"
	BadgeText    string            // "РУИНЕР", "ПОДОЗРЕНИЕ"
	Reasons      []string          // List of human-readable detected reasons
	MatchRuiners []MatchPlayerRuin // All ruiners/suspects detected across all players in the match
}

// RuinEvaluationInput contains match metrics needed for ruin scoring.
type RuinEvaluationInput struct {
	Duration     int      // match duration in seconds
	Kills        int
	Deaths       int
	Assists      int
	GPM          int
	XPM          int
	HeroDamage   int
	NetWorth     int
	LeaverStatus int
	Pings        int
	Items        []string // localized or standard item names
	IsWin        bool
}

// EvaluateRuin calculates a toxicity/ruining score from 0 to 100 based on OpenDota data.
// Thresholds: >= 65 is Ruiner, >= 45 is Suspected Ruiner.
func EvaluateRuin(in RuinEvaluationInput) RuinAnalysis {
	score := 0
	var reasons []string

	// 1. Leaver Status (Vector B / C)
	// OpenDota: 0=finished, 1=disconnected without abandon, 2=abandoned, 3=abandoned before FB, 4=abandoned after FB, 5=afk abandon
	if in.LeaverStatus >= 2 {
		score += 55
		reasons = append(reasons, "Ливнул из игры / AFK abandon")
	}

	// 2. Inventory Sabotage & Troll builds (Vector A)
	bootsCount := 0
	hasShadowAmulet := false
	for _, item := range in.Items {
		itemLower := strings.ToLower(item)
		if strings.Contains(itemLower, "boots") || strings.Contains(itemLower, "treads") || strings.Contains(itemLower, "greaves") {
			bootsCount++
		}
		if strings.Contains(itemLower, "shadow amulet") {
			hasShadowAmulet = true
		}
	}

	// Troll boots build
	if bootsCount >= 4 {
		score += 65
		reasons = append(reasons, fmt.Sprintf("Тролль-билд (%d сапог в инвентаре)", bootsCount))
	} else if bootsCount == 3 {
		score += 45
		reasons = append(reasons, "Тролль-билд (3 пары сапог в инвентаре)")
	} else if bootsCount == 2 && in.Duration > 900 {
		if !in.IsWin {
			score += 15
			reasons = append(reasons, "2 пары сапог в инвентаре")
		}
	}

	// Shadow Amulet AFK
	if hasShadowAmulet && in.Duration >= 1200 && (in.GPM < 300 || in.HeroDamage < 3000) {
		score += 65
		reasons = append(reasons, "AFK с Shadow Amulet")
	}

	// Sold / Destroyed items: late in game with empty or 1-item inventory and low net worth
	if in.Duration >= 1200 && len(in.Items) <= 1 && in.NetWorth > 0 && in.NetWorth < 2500 {
		score += 65
		reasons = append(reasons, "Сломал или продал все вещи")
	}

	// 3. Statistical Anomalies: Hero Damage & GPM (Vector B)
	if in.Duration >= 1200 {
		if in.HeroDamage > 0 && in.HeroDamage < 1500 {
			score += 30
			reasons = append(reasons, fmt.Sprintf("Критически низкий урон (%s)", formatNumberK(in.HeroDamage)))
		} else if in.Duration >= 1800 && in.HeroDamage > 0 && in.HeroDamage < 3500 {
			score += 20
			reasons = append(reasons, fmt.Sprintf("Аномально низкий урон (%s за %s)", formatNumberK(in.HeroDamage), formatDuration(in.Duration)))
		}
	}

	if in.Duration >= 900 {
		if in.GPM > 0 && in.GPM < 160 {
			score += 30
			reasons = append(reasons, fmt.Sprintf("Бойкот фарма / AFK (GPM: %d)", in.GPM))
		} else if in.GPM > 0 && in.GPM < 220 {
			score += 15
			reasons = append(reasons, fmt.Sprintf("Критически низкий GPM (%d)", in.GPM))
		}
	}

	// Intentional Feeding
	durationMins := float64(in.Duration) / 60.0
	if durationMins < 1.0 {
		durationMins = 1.0
	}
	deathsPerMin := float64(in.Deaths) / durationMins
	kdaNumerator := float64(in.Kills + in.Assists)
	kdaRatio := kdaNumerator / float64(max(1, in.Deaths))
	feedRatio := float64(in.Deaths) / float64(max(1, in.Kills+in.Assists))

	if in.Deaths >= 15 && in.Kills+in.Assists <= 3 {
		score += 40
		reasons = append(reasons, fmt.Sprintf("Умышленный фид (%d смертей при %d/%d/%d)", in.Deaths, in.Kills, in.Deaths, in.Assists))
	} else if in.Deaths >= 13 && kdaRatio < 0.3 {
		score += 30
		reasons = append(reasons, fmt.Sprintf("Аномальное количество смертей (%d смертей, KDA %.2f)", in.Deaths, kdaRatio))
	} else if in.Deaths >= 10 && feedRatio >= 4.0 {
		score += 20
		reasons = append(reasons, fmt.Sprintf("Высокий фид-рейт (%d смертей, K/D/A %d/%d/%d)", in.Deaths, in.Kills, in.Deaths, in.Assists))
	}

	if deathsPerMin >= 0.6 && in.Deaths >= 10 {
		score += 15
		reasons = append(reasons, fmt.Sprintf("Слишком частые смерти (%.1f/мин)", deathsPerMin))
	}

	// 4. Communication / Pings (Vector C)
	if in.Pings >= 60 {
		score += 15
		reasons = append(reasons, fmt.Sprintf("Спам пингами (%d пингов)", in.Pings))
	}

	// 5. Win Mitigation: reduce severity slightly if the team actually won
	if in.IsWin && in.LeaverStatus < 2 && bootsCount < 3 {
		score = int(float64(score) * 0.7)
	}

	if score > 100 {
		score = 100
	}

	res := RuinAnalysis{
		Score:   score,
		Reasons: reasons,
	}

	if score >= 65 {
		res.IsRuiner = true
		res.Verdict = "Руинер"
		res.BadgeText = "РУИНЕР"
	} else if score >= 45 {
		res.IsSuspect = true
		res.Verdict = "Подозрение на руин"
		res.BadgeText = "ПОДОЗРЕНИЕ"
	} else {
		res.Verdict = "Чисто"
	}

	return res
}

// EvaluateMatchRuiners evaluates all players in a match details response and returns
// a slice of players who were detected as ruiners or suspects, sorted by priority.
func EvaluateMatchRuiners(details matchDetails, trackedAccountID int64, trackedSlot int, heroes map[int]string, itemNames map[int]string) []MatchPlayerRuin {
	if len(details.Players) == 0 {
		return nil
	}

	if trackedSlot < 0 && trackedAccountID > 0 {
		for _, p := range details.Players {
			if p.AccountID == trackedAccountID {
				trackedSlot = p.PlayerSlot
				break
			}
		}
	}
	if trackedSlot < 0 {
		trackedSlot = 0
	}

	var ruiners []MatchPlayerRuin

	for _, p := range details.Players {
		heroName := heroes[p.HeroID]
		if heroName == "" {
			heroName = fmt.Sprintf("Hero #%d", p.HeroID)
		}

		pName := strings.TrimSpace(p.PersonaName)
		if pName == "" {
			pName = heroName
		}

		var items []string
		if itemNames != nil {
			items = collectPlayerItems(p, itemNames)
		}
		pIsWin := isWin(details.RadiantWin, p.PlayerSlot)

		analysis := EvaluateRuin(RuinEvaluationInput{
			Duration:     details.Duration,
			Kills:        p.Kills,
			Deaths:       p.Deaths,
			Assists:      p.Assists,
			GPM:          p.GPM,
			XPM:          p.XPM,
			HeroDamage:   p.HeroDamage,
			NetWorth:     p.NetWorth,
			LeaverStatus: p.LeaverStatus,
			Pings:        p.Pings,
			Items:        items,
			IsWin:        pIsWin,
		})

		if analysis.IsRuiner || analysis.IsSuspect {
			isTracked := (trackedAccountID > 0 && p.AccountID == trackedAccountID) || p.PlayerSlot == trackedSlot
			isAlly := (p.PlayerSlot < 128) == (trackedSlot < 128)

			ruiners = append(ruiners, MatchPlayerRuin{
				AccountID:  p.AccountID,
				PlayerName: pName,
				HeroID:     p.HeroID,
				HeroName:   heroName,
				PlayerSlot: p.PlayerSlot,
				IsTracked:  isTracked,
				IsAlly:     isAlly,
				Analysis:   analysis,
			})
		}
	}

	sort.SliceStable(ruiners, func(i, j int) bool {
		// 1. Tracked player first
		if ruiners[i].IsTracked != ruiners[j].IsTracked {
			return ruiners[i].IsTracked
		}
		// 2. Allies before enemies
		if ruiners[i].IsAlly != ruiners[j].IsAlly {
			return ruiners[i].IsAlly
		}
		// 3. Higher score first
		return ruiners[i].Analysis.Score > ruiners[j].Analysis.Score
	})

	return ruiners
}

// formatRuinAlert generates a formatted HTML alert for Telegram messages.
func formatRuinAlert(analysis RuinAnalysis) string {
	if len(analysis.MatchRuiners) > 0 {
		ruinerCount := 0
		for _, r := range analysis.MatchRuiners {
			if r.Analysis.IsRuiner {
				ruinerCount++
			}
		}

		var b strings.Builder
		if ruinerCount > 0 {
			if ruinerCount == 1 {
				b.WriteString("🚨 <b>В матче обнаружен руинер:</b>")
			} else {
				b.WriteString(fmt.Sprintf("🚨 <b>В матче обнаружены руинеры (%d):</b>", ruinerCount))
			}
		} else {
			if len(analysis.MatchRuiners) == 1 {
				b.WriteString("⚠️ <b>В матче обнаружено подозрение на руин:</b>")
			} else {
				b.WriteString(fmt.Sprintf("⚠️ <b>В матче обнаружены подозрения на руин (%d):</b>", len(analysis.MatchRuiners)))
			}
		}

		maxDisplay := 4
		for i, r := range analysis.MatchRuiners {
			if i >= maxDisplay {
				remaining := len(analysis.MatchRuiners) - maxDisplay
				b.WriteString(fmt.Sprintf("\n• <i>...и ещё %d %s (подробнее по кнопке ниже)</i>",
					remaining, pluralize(remaining, "игрок", "игрока", "игроков")))
				break
			}
			b.WriteString("\n")
			teamTag := "Союзник"
			if r.IsTracked {
				teamTag = "Игрок (Вы)"
			} else if !r.IsAlly {
				teamTag = "Противник"
			}
			badge := "РУИНЕР"
			if !r.Analysis.IsRuiner {
				badge = "ПОДОЗРЕНИЕ"
			}
			b.WriteString(fmt.Sprintf("• <b>[%s]</b> %s (%s) — <b>%s</b> (<code>%d/100</code>)",
				teamTag, escapeHTML(r.HeroName), escapeHTML(r.PlayerName), badge, r.Analysis.Score))
			if len(r.Analysis.Reasons) > 0 {
				b.WriteString("\n  ⚠️ <i>Причины: " + escapeHTML(strings.Join(r.Analysis.Reasons, ", ")) + "</i>")
			}
		}
		return b.String()
	}

	if analysis.IsRuiner {
		msg := fmt.Sprintf("🚨 <b>Вердикт: РУИНЕР</b> (индекс: <code>%d/100</code>)", analysis.Score)
		if len(analysis.Reasons) > 0 {
			msg += "\n⚠️ <i>Причины: " + escapeHTML(strings.Join(analysis.Reasons, ", ")) + "</i>"
		}
		return msg
	}
	if analysis.IsSuspect {
		msg := fmt.Sprintf("⚠️ <b>Подозрение на руин</b> (индекс: <code>%d/100</code>)", analysis.Score)
		if len(analysis.Reasons) > 0 {
			msg += "\n⚠️ <i>Причины: " + escapeHTML(strings.Join(analysis.Reasons, ", ")) + "</i>"
		}
		return msg
	}
	return ""
}
