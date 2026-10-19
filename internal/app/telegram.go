package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type telegramUpdate struct {
	UpdateID      int                    `json:"update_id"`
	Message       *telegramMessage       `json:"message"`
	CallbackQuery *telegramCallbackQuery `json:"callback_query"`
}

type telegramMessage struct {
	MessageID int          `json:"message_id"`
	Chat      telegramChat `json:"chat"`
	Text      string       `json:"text"`
}

type telegramChat struct {
	ID int64 `json:"id"`
}

type telegramCallbackQuery struct {
	ID      string           `json:"id"`
	Data    string           `json:"data"`
	Message *telegramMessage `json:"message"`
}

type telegramUpdatesResponse struct {
	OK          bool             `json:"ok"`
	Result      []telegramUpdate `json:"result"`
	Description string           `json:"description"`
}

func runTelegramBot(token string, accountStore *accountIDStore, heroes map[int]string) error {
	apiBase := fmt.Sprintf(telegramBaseURL, token)
	offset := 0
	for {
		url := fmt.Sprintf("%s/getUpdates?timeout=30&offset=%d", apiBase, offset)
		var resp telegramUpdatesResponse
		if err := getJSON(url, &resp, nil); err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if !resp.OK {
			return fmt.Errorf("telegram getUpdates failed: %s", resp.Description)
		}
		for _, upd := range resp.Result {
			offset = upd.UpdateID + 1
			if upd.CallbackQuery != nil {
				if err := handleTelegramCallback(apiBase, upd.CallbackQuery, heroes); err != nil {
					return err
				}
				continue
			}
			if upd.Message == nil {
				continue
			}
			text := strings.TrimSpace(upd.Message.Text)
			if isStatCommand(text) {
				accountIDs := accountStore.Get()
				for _, accountID := range accountIDs {
					player, err := fetchPlayerProfile(accountID)
					if err != nil {
						sendTelegramMessage(apiBase, upd.Message.Chat.ID, formatOpenDotaError(err), "", nil)
						break
					}
					matches, err := fetchRecentMatches(accountID)
					if err != nil {
						sendTelegramMessage(apiBase, upd.Message.Chat.ID, formatOpenDotaError(err), "", nil)
						break
					}
					winrate := calcWinrate(matches, 20)
					isWin, streak := calcStreak(matches)
					streakBadge := formatStreakBadge(isWin, streak, true)
					if len(matches) > 10 {
						matches = matches[:10]
					}
					table := buildPlayerTable(matches, heroes, player.PersonaName)
					name := player.PersonaName
					if name == "" {
						name = "неизвестный"
					}
					var header string
					if streakBadge != "" {
						header = fmt.Sprintf("<b>Последние матчи (%s)</b>\n%s\n<b>Winrate (за 20 игр): %.1f%%</b>\n<b>✅ победа, ❌ поражение</b>\n", escapeHTML(name), streakBadge, winrate)
					} else {
						header = fmt.Sprintf("<b>Последние матчи (%s)</b>\n<b>Winrate (за 20 игр): %.1f%%</b>\n<b>✅ победа, ❌ поражение</b>\n", escapeHTML(name), winrate)
					}
					if len(matches) > 0 {
						cardBytes, ruinAnalysis, err := buildMatchCard(accountID, matches[0], player.PersonaName, player.AvatarFull, heroes, isWin, streak)
						if err == nil && len(cardBytes) > 0 {
							cardCaption := header
							if alert := formatRuinAlert(ruinAnalysis); alert != "" {
								cardCaption += "\n" + alert
							}
							markup := buildMatchDetailsMarkup(matchNotification{MatchID: matches[0].MatchID, AccountID: accountID})
							if err := sendTelegramPhotoBytes(apiBase, upd.Message.Chat.ID, cardBytes, cardCaption, "HTML", markup); err == nil {
								continue
							}
						}
					}
					if player.AvatarFull != "" {
						if err := sendTelegramPhoto(apiBase, upd.Message.Chat.ID, player.AvatarFull, header, "HTML", nil); err != nil {
							return err
						}
						for _, msg := range buildTelegramMessages(table, "") {
							if err := sendTelegramMessage(apiBase, upd.Message.Chat.ID, msg, "HTML", nil); err != nil {
								return err
							}
						}
					} else {
						for _, msg := range buildTelegramMessages(table, header) {
							if err := sendTelegramMessage(apiBase, upd.Message.Chat.ID, msg, "HTML", nil); err != nil {
								return err
							}
						}
					}
				}
				continue
			}
			if isRatingCommand(text) {
				cardBytes, err := buildRatingCardImage(accountStore.Get())
				if err == nil && len(cardBytes) > 0 {
					if err := sendTelegramPhotoBytes(apiBase, upd.Message.Chat.ID, cardBytes, "", "", nil); err == nil {
						continue
					}
				}
				table, err := buildRatingTable(accountStore.Get())
				if err != nil {
					sendTelegramMessage(apiBase, upd.Message.Chat.ID, formatOpenDotaError(err), "", nil)
					continue
				}
				header := "<b>Рейтинг по Winrate (50)</b>\n"
				for _, msg := range buildTelegramMessages(table, header) {
					if err := sendTelegramMessage(apiBase, upd.Message.Chat.ID, msg, "HTML", nil); err != nil {
						return err
					}
				}
				continue
			}
			if ok, limit, err := parseFriendsCommand(text); ok {
				if err != nil {
					sendTelegramMessage(apiBase, upd.Message.Chat.ID, fmt.Sprintf("Ошибка: %s", err.Error()), "", nil)
					continue
				}
				table, err := buildBestFriendsTable(accountStore.Get(), limit)
				if err != nil {
					sendTelegramMessage(apiBase, upd.Message.Chat.ID, formatOpenDotaError(err), "", nil)
					continue
				}
				header := fmt.Sprintf("<b>Лучшие напарники по Winrate (за последние %d игр)</b>\n", limit)
				for _, msg := range buildTelegramMessages(table, header) {
					if err := sendTelegramMessage(apiBase, upd.Message.Chat.ID, msg, "HTML", nil); err != nil {
						return err
					}
				}
				continue
			}
			if isChatIDCommand(text) {
				if err := sendTelegramMessage(apiBase, upd.Message.Chat.ID, fmt.Sprintf("chat_id: %d", upd.Message.Chat.ID), "", nil); err != nil {
					return err
				}
				continue
			}
			if isTestCommand(text) {
				msg, err := buildTestMatchSummary(accountStore.Get(), heroes)
				if err != nil {
					sendTelegramMessage(apiBase, upd.Message.Chat.ID, fmt.Sprintf("Ошибка: %s", err.Error()), "", nil)
					continue
				}
				markup := buildMatchDetailsMarkup(msg)
				if len(msg.PhotoData) > 0 {
					if err := sendTelegramPhotoBytes(apiBase, upd.Message.Chat.ID, msg.PhotoData, msg.Text, msg.ParseMode, markup); err == nil {
						continue
					} else if msg.ParseMode != "" {
						if err := sendTelegramPhotoBytes(apiBase, upd.Message.Chat.ID, msg.PhotoData, stripHTML(msg.Text), "", markup); err == nil {
							continue
						}
					}
				}
				if err := sendTelegramMessage(apiBase, upd.Message.Chat.ID, msg.Text, msg.ParseMode, markup); err != nil {
					if msg.ParseMode != "" {
						if err2 := sendTelegramMessage(apiBase, upd.Message.Chat.ID, stripHTML(msg.Text), "", markup); err2 == nil {
							continue
						}
					}
					return err
				}
				continue
			}
			if isReloadAccsCommand(text) {
				ids, err := loadAccountIDs("account_id")
				if err != nil {
					sendTelegramMessage(apiBase, upd.Message.Chat.ID, fmt.Sprintf("Ошибка reload: %s", err.Error()), "", nil)
					continue
				}
				accountStore.Set(ids)
				if err := sendTelegramMessage(apiBase, upd.Message.Chat.ID, fmt.Sprintf("account_id обновлён: %d аккаунтов", len(ids)), "", nil); err != nil {
					return err
				}
				continue
			}
		}
	}
}

func sendPostJSON(url string, body []byte) error {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create telegram post request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram post request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read telegram response: %w", err)
	}

	type tgBasicResponse struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	var tgResp tgBasicResponse
	if err := json.Unmarshal(respData, &tgResp); err == nil {
		if !tgResp.OK {
			return fmt.Errorf("telegram failed: %s", tgResp.Description)
		}
	}
	return nil
}

func sendTelegramMessage(apiBase string, chatID int64, text string, parseMode string, replyMarkup any) error {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	if parseMode != "" {
		payload["parse_mode"] = parseMode
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram message: %w", err)
	}
	return sendPostJSON(apiBase+"/sendMessage", body)
}

func sendTelegramPhoto(apiBase string, chatID int64, photoURL string, caption string, parseMode string, replyMarkup any) error {
	payload := map[string]any{
		"chat_id": chatID,
		"photo":   photoURL,
	}
	if caption != "" {
		payload["caption"] = caption
	}
	if parseMode != "" {
		payload["parse_mode"] = parseMode
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram photo: %w", err)
	}
	return sendPostJSON(apiBase+"/sendPhoto", body)
}

func sendTelegramPhotoBytes(apiBase string, chatID int64, photoData []byte, caption string, parseMode string, replyMarkup any) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return fmt.Errorf("write chat_id: %w", err)
	}

	part, err := writer.CreateFormFile("photo", "card.png")
	if err != nil {
		return fmt.Errorf("create form photo file: %w", err)
	}
	if _, err := part.Write(photoData); err != nil {
		return fmt.Errorf("write photo bytes: %w", err)
	}

	if parseMode != "" {
		if err := writer.WriteField("parse_mode", parseMode); err != nil {
			return fmt.Errorf("write parse_mode: %w", err)
		}
	}

	if caption != "" {
		if err := writer.WriteField("caption", caption); err != nil {
			return fmt.Errorf("write caption: %w", err)
		}
	}

	if replyMarkup != nil {
		if b, err := json.Marshal(replyMarkup); err == nil {
			if err := writer.WriteField("reply_markup", string(b)); err != nil {
				return fmt.Errorf("write reply_markup: %w", err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiBase+"/sendPhoto", &body)
	if err != nil {
		return fmt.Errorf("create sendPhoto request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("sendPhoto request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read sendPhoto response: %w", err)
	}

	type tgBasicResponse struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	var tgResp tgBasicResponse
	if err := json.Unmarshal(respData, &tgResp); err == nil {
		if !tgResp.OK {
			return fmt.Errorf("telegram failed: %s", tgResp.Description)
		}
	}
	return nil
}

func splitText(text string, maxLen int) []string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return []string{text}
	}
	var parts []string
	for len(runes) > maxLen {
		parts = append(parts, string(runes[:maxLen]))
		runes = runes[maxLen:]
	}
	if len(runes) > 0 {
		parts = append(parts, string(runes))
	}
	return parts
}

func escapeHTML(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))
	for _, r := range text {
		switch r {
		case '&':
			builder.WriteString("&amp;")
		case '<':
			builder.WriteString("&lt;")
		case '>':
			builder.WriteString("&gt;")
		default:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func buildTelegramMessages(report string, header string) []string {
	escaped := escapeHTML(report)

	preWrapperLen := runeLen("<pre></pre>")
	firstMax := telegramMaxLen - runeLen(header) - preWrapperLen
	if firstMax < 100 {
		firstMax = 100
	}
	if runeLen(escaped) == 0 {
		return []string{header + "<pre></pre>"}
	}

	firstParts := splitText(escaped, firstMax)
	first := firstParts[0]
	escapedRunes := []rune(escaped)
	firstLen := runeLen(first)
	escaped = string(escapedRunes[firstLen:])

	messages := []string{header + "<pre>" + first + "</pre>"}
	for runeLen(escaped) > 0 {
		parts := splitText(escaped, telegramMaxLen-preWrapperLen)
		part := parts[0]
		messages = append(messages, "<pre>"+part+"</pre>")
		partLen := runeLen(part)
		escapedRunes = []rune(escaped)
		escaped = string(escapedRunes[partLen:])
	}
	return messages
}

func runeLen(value string) int {
	return len([]rune(value))
}

func telegramNotifier(token string) func(matchNotification) {
	chatIDRaw := strings.TrimSpace(os.Getenv(telegramChatEnv))
	if chatIDRaw == "" {
		return nil
	}
	chatID, err := strconv.ParseInt(chatIDRaw, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid %s: %s\n", telegramChatEnv, err.Error())
		return nil
	}
	apiBase := fmt.Sprintf(telegramBaseURL, token)
	return func(msg matchNotification) {
		replyMarkup := buildMatchDetailsMarkup(msg)
		if len(msg.PhotoData) > 0 {
			if err := sendTelegramPhotoBytes(apiBase, chatID, msg.PhotoData, msg.Text, msg.ParseMode, replyMarkup); err == nil {
				return
			} else {
				fmt.Fprintf(os.Stderr, "telegram notify photo error: %s\n", err.Error())
				if msg.ParseMode != "" {
					if err := sendTelegramPhotoBytes(apiBase, chatID, msg.PhotoData, stripHTML(msg.Text), "", replyMarkup); err == nil {
						return
					}
				}
			}
		}
		if err := sendTelegramMessage(apiBase, chatID, msg.Text, msg.ParseMode, replyMarkup); err != nil {
			fmt.Fprintf(os.Stderr, "telegram notify error: %s\n", err.Error())
			if msg.ParseMode != "" {
				_ = sendTelegramMessage(apiBase, chatID, stripHTML(msg.Text), "", replyMarkup)
			}
		}
	}
}

func stripHTML(input string) string {
	var builder strings.Builder
	inTag := false
	for _, r := range input {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			builder.WriteRune(r)
		}
	}
	res := builder.String()
	res = strings.ReplaceAll(res, "&lt;", "<")
	res = strings.ReplaceAll(res, "&gt;", ">")
	res = strings.ReplaceAll(res, "&amp;", "&")
	return res
}

func buildMatchDetailsMarkup(msg matchNotification) any {
	if msg.MatchID == 0 || msg.AccountID == 0 {
		return nil
	}
	return map[string]any{
		"inline_keyboard": [][]map[string]string{{
			{
				"text":          "Подробнее",
				"callback_data": fmt.Sprintf("match:%d:%d", msg.AccountID, msg.MatchID),
			},
		}},
	}
}

func handleTelegramCallback(apiBase string, query *telegramCallbackQuery, heroes map[int]string) error {
	if query == nil {
		return nil
	}
	if err := answerTelegramCallback(apiBase, query.ID, "Загружаю детали матча"); err != nil {
		return err
	}
	accountID, matchID, ok := parseMatchCallbackData(query.Data)
	if !ok {
		return nil
	}
	if query.Message == nil {
		return nil
	}
	details, err := fetchMatchDetails(matchID)
	if err != nil {
		return sendTelegramMessage(apiBase, query.Message.Chat.ID, fmt.Sprintf("Ошибка: %s", err.Error()), "", nil)
	}
	itemNames, err := fetchItemNames()
	if err != nil {
		return sendTelegramMessage(apiBase, query.Message.Chat.ID, fmt.Sprintf("Ошибка: %s", err.Error()), "", nil)
	}
	text, err := formatMatchDetailsMessage(details, accountID, heroes, itemNames)
	if err != nil {
		return sendTelegramMessage(apiBase, query.Message.Chat.ID, fmt.Sprintf("Ошибка: %s", err.Error()), "", nil)
	}
	return sendTelegramMessage(apiBase, query.Message.Chat.ID, text, "HTML", nil)
}

func parseMatchCallbackData(data string) (int64, int64, bool) {
	parts := strings.Split(data, ":")
	if len(parts) != 3 || parts[0] != "match" {
		return 0, 0, false
	}
	accountID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	matchID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return accountID, matchID, true
}

func answerTelegramCallback(apiBase string, callbackID string, text string) error {
	payload := map[string]any{
		"callback_query_id": callbackID,
	}
	if strings.TrimSpace(text) != "" {
		payload["text"] = text
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram callback answer: %w", err)
	}
	return sendPostJSON(apiBase+"/answerCallbackQuery", body)
}
