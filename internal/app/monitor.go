package app

import (
	"fmt"
	"os"
	"time"
)

func monitorMatches(accountStore *accountIDStore, heroes map[int]string, notify func(matchNotification)) {
	accountIDs := accountStore.Get()
	lastMatch := make(map[int64]int64, len(accountIDs))
	names := make(map[int64]string, len(accountIDs))
	avatars := make(map[int64]string, len(accountIDs))
	var startupStreaks []playerStreak
	for _, accountID := range accountIDs {
		player, err := fetchPlayerProfile(accountID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "profile error: %s\n", err.Error())
		} else {
			names[accountID] = fallbackName(player.PersonaName)
			avatars[accountID] = player.AvatarFull
		}
		matches, err := fetchRecentMatches(accountID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "matches error: %s\n", err.Error())
			continue
		}
		if len(matches) > 0 {
			lastMatch[accountID] = matches[0].MatchID
			isWin, streak := calcStreak(matches)
			if streak >= 3 {
				startupStreaks = append(startupStreaks, playerStreak{
					Name:   names[accountID],
					IsWin:  isWin,
					Streak: streak,
				})
			}
		}
	}

	if len(startupStreaks) > 0 {
		plainText := formatStartupStreaksMessage(startupStreaks, false)
		fmt.Println(plainText)
		if notify != nil {
			notify(matchNotification{
				Text:      formatStartupStreaksMessage(startupStreaks, true),
				ParseMode: "HTML",
			})
		}
	}

	if notify == nil {
		notify = func(msg matchNotification) {
			fmt.Println(msg.Text)
		}
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		accountIDs = accountStore.Get()
		for _, accountID := range accountIDs {
			if _, ok := names[accountID]; !ok {
				player, err := fetchPlayerProfile(accountID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "profile error: %s\n", err.Error())
				} else {
					names[accountID] = fallbackName(player.PersonaName)
					avatars[accountID] = player.AvatarFull
				}
			}
			matches, err := fetchRecentMatches(accountID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "matches error: %s\n", err.Error())
				continue
			}
			if len(matches) == 0 {
				continue
			}
			prev, ok := lastMatch[accountID]
			if !ok {
				lastMatch[accountID] = matches[0].MatchID
				continue
			}
			if matches[0].MatchID == prev {
				continue
			}
			var newMatches []recentMatch
			for _, m := range matches {
				if m.MatchID == prev {
					break
				}
				newMatches = append(newMatches, m)
			}
			for i := len(newMatches) - 1; i >= 0; i-- {
				summary := formatMatchSummary(names[accountID], newMatches[i], heroes)
				isWin, streak := calcStreak(matches[i:])
				if badge := formatStreakBadge(isWin, streak, true); badge != "" {
					summary += "\n" + badge
				}
				cardData, ruinAnalysis, _ := buildMatchCard(accountID, newMatches[i], names[accountID], avatars[accountID], heroes, isWin, streak)
				if alert := formatRuinAlert(ruinAnalysis); alert != "" {
					summary += "\n" + alert
				}
				notify(matchNotification{
					Text:      summary,
					MatchID:   newMatches[i].MatchID,
					AccountID: accountID,
					ParseMode: "HTML",
					PhotoData: cardData,
				})
			}
			lastMatch[accountID] = matches[0].MatchID
		}
	}
}
