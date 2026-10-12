package app

import (
	"fmt"
	"strings"
)

func matchWin(m recentMatch) bool {
	return isWin(m.RadiantWin, m.PlayerSlot)
}

func calcWinrate(matches []recentMatch, limit int) float64 {
	winrate, _ := calcWinrateWithCount(matches, limit)
	return winrate
}

func calcWinrateWithCount(matches []recentMatch, limit int) (float64, int) {
	if limit <= 0 {
		return 0, 0
	}
	total := 0
	wins := 0
	for _, m := range matches {
		if total >= limit {
			break
		}
		total++
		if matchWin(m) {
			wins++
		}
	}
	if total == 0 {
		return 0, 0
	}
	return float64(wins) * 100 / float64(total), total
}

func calcWinrateFromMatches(matches []playerMatch) (float64, int) {
	if len(matches) == 0 {
		return 0, 0
	}
	wins := 0
	for _, m := range matches {
		if isWin(m.RadiantWin, m.PlayerSlot) {
			wins++
		}
	}
	return float64(wins) * 100 / float64(len(matches)), len(matches)
}

func isWin(radiantWin bool, playerSlot int) bool {
	isRadiant := playerSlot < 128
	return (radiantWin && isRadiant) || (!radiantWin && !isRadiant)
}

func formatDuration(seconds int) string {
	if seconds <= 0 {
		return "0:00"
	}
	min := seconds / 60
	sec := seconds % 60
	return fmt.Sprintf("%d:%02d", min, sec)
}

func trimTo(value string, max int) string {
	if len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func fallbackName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "неизвестный"
	}
	return strings.TrimSpace(name)
}

func calcStreak(matches []recentMatch) (isWin bool, count int) {
	if len(matches) == 0 {
		return false, 0
	}
	firstWin := matchWin(matches[0])
	streak := 0
	for _, m := range matches {
		if matchWin(m) == firstWin {
			streak++
		} else {
			break
		}
	}
	return firstWin, streak
}

func pluralize(n int, one, twoFour, many string) string {
	mod10 := n % 10
	mod100 := n % 100
	if mod100 >= 11 && mod100 <= 19 {
		return many
	}
	if mod10 == 1 {
		return one
	}
	if mod10 >= 2 && mod10 <= 4 {
		return twoFour
	}
	return many
}

func formatStreakBadge(isWin bool, count int, html bool) string {
	if count < 3 {
		return ""
	}
	if isWin {
		word := pluralize(count, "победа", "победы", "побед")
		switch {
		case count == 3:
			if html {
				return fmt.Sprintf("🔥 <b>СЕРИЯ ПОБЕД: %d %s подряд!</b> 🔥", count, word)
			}
			return fmt.Sprintf("🔥 СЕРИЯ ПОБЕД: %d %s подряд! 🔥", count, word)
		case count == 4:
			if html {
				return fmt.Sprintf("🔥🚀 <b>ОГНЕННЫЙ ВИНСТРИК: %d %s подряд!</b> 🚀🔥", count, word)
			}
			return fmt.Sprintf("🔥🚀 ОГНЕННЫЙ ВИНСТРИК: %d %s подряд! 🚀🔥", count, word)
		default:
			if html {
				return fmt.Sprintf("👑⚡ <b>НЕОСТАНОВИМ: %d %s подряд!</b> ⚡👑", count, word)
			}
			return fmt.Sprintf("👑⚡ НЕОСТАНОВИМ: %d %s подряд! ⚡👑", count, word)
		}
	} else {
		word := pluralize(count, "поражение", "поражения", "поражений")
		switch {
		case count == 3:
			if html {
				return fmt.Sprintf("💀 <b>СЕРИЯ ПОРАЖЕНИЙ: %d %s подряд!</b> 💀", count, word)
			}
			return fmt.Sprintf("💀 СЕРИЯ ПОРАЖЕНИЙ: %d %s подряд! 💀", count, word)
		case count == 4:
			if html {
				return fmt.Sprintf("🧊📉 <b>ЛУЗСТРИК: %d %s подряд!</b> 📉🧊", count, word)
			}
			return fmt.Sprintf("🧊📉 ЛУЗСТРИК: %d %s подряд! 📉🧊", count, word)
		default:
			if html {
				return fmt.Sprintf("🚨☠️ <b>ЖЁСТКИЙ ТИЛЬТ: %d %s подряд!</b> ☠️🚨", count, word)
			}
			return fmt.Sprintf("🚨☠️ ЖЁСТКИЙ ТИЛЬТ: %d %s подряд! ☠️🚨", count, word)
		}
	}
}

type playerStreak struct {
	Name   string
	IsWin  bool
	Streak int
}

func formatStartupStreaksMessage(streaks []playerStreak, html bool) string {
	if len(streaks) == 0 {
		return ""
	}
	var b strings.Builder
	title := "⚡ Активные серии при запуске:"
	if len(streaks) == 1 {
		title = "⚡ Активная серия при запуске:"
	}
	if html {
		b.WriteString("<b>" + title + "</b>\n\n")
	} else {
		b.WriteString(title + "\n\n")
	}

	for i, s := range streaks {
		badge := formatStreakBadge(s.IsWin, s.Streak, html)
		name := fallbackName(s.Name)
		if html {
			b.WriteString(fmt.Sprintf("👤 <b>%s</b>\n%s", escapeHTML(name), badge))
		} else {
			b.WriteString(fmt.Sprintf("👤 %s\n%s", name, badge))
		}
		if i < len(streaks)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

