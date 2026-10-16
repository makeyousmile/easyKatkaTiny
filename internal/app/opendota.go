package app

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed assets/heroes.json
var embeddedHeroesJSON []byte

//go:embed assets/items.json
var embeddedItemsJSON []byte

type hero struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LocalizedName string `json:"localized_name"`
}

var (
	heroShortNamesMu sync.RWMutex
	heroShortNames   = make(map[int]string)
)

func setHeroShortName(id int, name string) {
	heroShortNamesMu.Lock()
	defer heroShortNamesMu.Unlock()
	heroShortNames[id] = strings.TrimPrefix(name, "npc_dota_hero_")
}

func getHeroShortName(id int) string {
	heroShortNamesMu.RLock()
	defer heroShortNamesMu.RUnlock()
	return heroShortNames[id]
}

type recentMatch struct {
	MatchID     int64 `json:"match_id"`
	HeroID      int   `json:"hero_id"`
	Kills       int   `json:"kills"`
	Deaths      int   `json:"deaths"`
	Assists     int   `json:"assists"`
	Duration    int   `json:"duration"`
	StartTime   int64 `json:"start_time"`
	PlayerSlot  int   `json:"player_slot"`
	RadiantWin  bool  `json:"radiant_win"`
	HeroDamage  int   `json:"hero_damage"`
	GoldPerMin  int   `json:"gold_per_min"`
	TowerDamage  int   `json:"tower_damage"`
	LastHits     int   `json:"last_hits"`
	LeaverStatus int   `json:"leaver_status"`
}

type playerProfile struct {
	Profile struct {
		PersonaName string `json:"personaname"`
		AvatarFull  string `json:"avatarfull"`
	} `json:"profile"`
}

type playerProfileData struct {
	PersonaName string
	AvatarFull  string
}

type peerEntry struct {
	AccountID   int64  `json:"account_id"`
	Personaname string `json:"personaname"`
	WithGames   int    `json:"with_games"`
	WithWin     int    `json:"with_win"`
}

type playerMatch struct {
	MatchID    int64 `json:"match_id"`
	PlayerSlot int   `json:"player_slot"`
	RadiantWin bool  `json:"radiant_win"`
}

type matchDetails struct {
	MatchID      int64                `json:"match_id"`
	Duration     int                  `json:"duration"`
	StartTime    int64                `json:"start_time"`
	GameMode     int                  `json:"game_mode"`
	LobbyType    int                  `json:"lobby_type"`
	RadiantWin   bool                 `json:"radiant_win"`
	RadiantScore int                  `json:"radiant_score"`
	DireScore    int                  `json:"dire_score"`
	FirstBlood   int                  `json:"first_blood_time"`
	LeagueName   string               `json:"league_name"`
	Players      []matchDetailsPlayer `json:"players"`
}

type matchDetailsPlayer struct {
	AccountID    int64  `json:"account_id"`
	PersonaName  string `json:"personaname"`
	HeroID       int    `json:"hero_id"`
	PlayerSlot   int    `json:"player_slot"`
	Kills        int    `json:"kills"`
	Deaths       int    `json:"deaths"`
	Assists      int    `json:"assists"`
	Level        int    `json:"level"`
	GPM          int    `json:"gold_per_min"`
	XPM          int    `json:"xp_per_min"`
	LastHits     int    `json:"last_hits"`
	Denies       int    `json:"denies"`
	HeroDamage   int    `json:"hero_damage"`
	TowerDamage  int    `json:"tower_damage"`
	HeroHealing  int    `json:"hero_healing"`
	NetWorth     int    `json:"net_worth"`
	Item0        int    `json:"item_0"`
	Item1        int    `json:"item_1"`
	Item2        int    `json:"item_2"`
	Item3        int    `json:"item_3"`
	Item4        int    `json:"item_4"`
	Item5        int    `json:"item_5"`
	Backpack0    int    `json:"backpack_0"`
	Backpack1    int    `json:"backpack_1"`
	Backpack2    int    `json:"backpack_2"`
	NeutralItem  int    `json:"item_neutral"`
	Pings        int    `json:"pings"`
	LeaverStatus int    `json:"leaver_status"`
}

type itemConstantsEntry struct {
	ID   int    `json:"id"`
	DName string `json:"dname"`
}

type rateLimiter struct {
	tick <-chan time.Time
}

func newRateLimiter(max int, per time.Duration) *rateLimiter {
	if max <= 0 {
		return nil
	}
	interval := per / time.Duration(max)
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	return &rateLimiter{tick: ticker.C}
}

func (l *rateLimiter) Wait() {
	if l == nil {
		return
	}
	<-l.tick
}

func loadFallbackHeroes() map[int]string {
	var heroes map[string]hero
	if err := json.Unmarshal(embeddedHeroesJSON, &heroes); err != nil {
		log.Printf("failed to parse embedded heroes: %v", err)
		return make(map[int]string)
	}
	result := make(map[int]string, len(heroes))
	for _, h := range heroes {
		if h.Name != "" {
			setHeroShortName(h.ID, h.Name)
		}
		if h.LocalizedName != "" {
			result[h.ID] = h.LocalizedName
		}
	}
	return result
}

func loadFallbackItems() map[int]string {
	var items map[string]itemConstantsEntry
	if err := json.Unmarshal(embeddedItemsJSON, &items); err != nil {
		log.Printf("failed to parse embedded items: %v", err)
		return make(map[int]string)
	}
	result := make(map[int]string, len(items))
	for _, item := range items {
		if item.ID == 0 || strings.TrimSpace(item.DName) == "" {
			continue
		}
		result[item.ID] = strings.TrimSpace(item.DName)
	}
	return result
}

func init() {
	loadFallbackHeroes()
}

func fetchHeroes() (map[int]string, error) {
	fallback := loadFallbackHeroes()
	if len(fallback) > 0 {
		return fallback, nil
	}
	var heroes []hero
	if err := getOpendotaJSON(baseURL+heroesURL, &heroes); err != nil {
		return nil, err
	}
	result := make(map[int]string, len(heroes))
	for _, h := range heroes {
		if h.Name != "" {
			setHeroShortName(h.ID, h.Name)
		}
		if h.LocalizedName == "" {
			continue
		}
		result[h.ID] = h.LocalizedName
	}
	return result, nil
}

func fetchRecentMatches(accountID int64) ([]recentMatch, error) {
	var matches []recentMatch
	url := fmt.Sprintf(baseURL+recentMatchesURL, accountID)
	if err := getOpendotaJSON(url, &matches); err != nil {
		return nil, err
	}
	return matches, nil
}

func fetchPlayerMatches(accountID int64, limit int) ([]recentMatch, error) {
	if limit <= 0 {
		return []recentMatch{}, nil
	}
	var matches []recentMatch
	url := fmt.Sprintf("%s"+playerMatchesURL+"?limit=%d", baseURL, accountID, limit)
	if err := getOpendotaJSON(url, &matches); err != nil {
		return nil, err
	}
	return matches, nil
}

func fetchPlayerProfile(accountID int64) (playerProfileData, error) {
	var player playerProfile
	url := fmt.Sprintf(baseURL+playerURL, accountID)
	if err := getOpendotaJSON(url, &player); err != nil {
		return playerProfileData{}, err
	}
	return playerProfileData{
		PersonaName: strings.TrimSpace(player.Profile.PersonaName),
		AvatarFull:  strings.TrimSpace(player.Profile.AvatarFull),
	}, nil
}

func fetchPeers(accountID int64) ([]peerEntry, error) {
	var peers []peerEntry
	url := fmt.Sprintf(baseURL+peersURL, accountID)
	if err := getOpendotaJSON(url, &peers); err != nil {
		return nil, err
	}
	return peers, nil
}

func fetchMatchesWith(accountID int64, includedAccountID int64, limit int) ([]playerMatch, error) {
	var matches []playerMatch
	url := fmt.Sprintf("%s"+playerMatchesURL+"?included_account_id=%d&limit=%d", baseURL, accountID, includedAccountID, limit)
	if err := getOpendotaJSON(url, &matches); err != nil {
		return nil, err
	}
	return matches, nil
}

func fetchMatchDetails(matchID int64) (matchDetails, error) {
	var details matchDetails
	url := fmt.Sprintf(baseURL+matchURL, matchID)
	if err := getOpendotaJSON(url, &details); err != nil {
		return matchDetails{}, err
	}
	return details, nil
}

func fetchItemNames() (map[int]string, error) {
	var items map[string]itemConstantsEntry
	if err := getOpendotaJSON(baseURL+itemsURL, &items); err != nil {
		return nil, err
	}
	result := make(map[int]string, len(items))
	for _, item := range items {
		if item.ID == 0 || strings.TrimSpace(item.DName) == "" {
			continue
		}
		result[item.ID] = strings.TrimSpace(item.DName)
	}
	return result, nil
}

var (
	itemNamesCacheMu sync.RWMutex
	itemNamesCache   map[int]string
)

func getItemNames() (map[int]string, error) {
	itemNamesCacheMu.RLock()
	if len(itemNamesCache) > 0 {
		defer itemNamesCacheMu.RUnlock()
		return itemNamesCache, nil
	}
	itemNamesCacheMu.RUnlock()

	itemNamesCacheMu.Lock()
	defer itemNamesCacheMu.Unlock()
	if len(itemNamesCache) > 0 {
		return itemNamesCache, nil
	}
	fallback := loadFallbackItems()
	if len(fallback) > 0 {
		itemNamesCache = fallback
		return fallback, nil
	}
	names, err := fetchItemNames()
	if err != nil {
		return nil, err
	}
	itemNamesCache = names
	return names, nil
}

func sortItemNames(items []string) []string {
	filtered := items[:0]
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.Strings(filtered)
	return filtered
}

var sharedHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

func formatOpenDotaError(err error) string {
	if err == nil {
		return ""
	}
	errStr := err.Error()
	if strings.Contains(errStr, "Client.Timeout") || strings.Contains(errStr, "deadline exceeded") || strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "Network is unreachable") {
		return "⚠️ Сервер OpenDota временно не отвечает (таймаут). Пожалуйста, попробуйте чуть позже."
	}
	return fmt.Sprintf("Ошибка: %s", errStr)
}

func getOpendotaJSON(url string, out any) error {
	return getJSON(url, out, opendotaLimiter)
}

func getJSON(url string, out any, limiter *rateLimiter) error {
	if limiter != nil {
		limiter.Wait()
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "easyKatka/2.0")

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request %s returned status %d: %s", url, resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}
