package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AppVersion is the current version of the easyKatka bot.
const AppVersion = "v1.2.0"

// VersionChangelog is sent when a new version of the bot is launched.
const VersionChangelog = `🚀 <b>EasyKatka обновлён до версии ` + AppVersion + `!</b>

<b>Что нового:</b>
• 🎨 <b>Только красивая инфографика:</b> убрано дублирование результатов матча текстом — результат приходит в виде аккуратной графической карточки.
• 🚨 <b>Детекция руинеров:</b> анализ всех 10 игроков матча (фид, AFK с Shadow Amulet, тролль-билды, уничтожение вещей). При наличии нарушителей выводится отчёт с причинами.
• 📊 <b>Кнопка «Подробнее»:</b> по нажатию доступна полная статистика матча, KDA, нетворс, урон и инвентарь всех участников.
• ⚡️ <b>Оповещения об обновлениях:</b> теперь бот автоматически сообщает о выходе новой версии и её возможностях.`

const defaultVersionFilePath = "data/version.txt"

func getStoredVersion(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func saveStoredVersion(filePath string, version string) error {
	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(filePath, []byte(strings.TrimSpace(version)), 0644)
}

func checkAndNotifyNewVersion(notify func(matchNotification), filePath string) bool {
	if filePath == "" {
		filePath = defaultVersionFilePath
	}
	stored := getStoredVersion(filePath)
	if stored == AppVersion {
		return false
	}

	fmt.Printf("EasyKatka: new version detected: %s (previously: %q)\n", AppVersion, stored)
	if notify != nil {
		notify(matchNotification{
			Text:      VersionChangelog,
			ParseMode: "HTML",
		})
	}

	if err := saveStoredVersion(filePath, AppVersion); err != nil {
		fmt.Fprintf(os.Stderr, "failed to save version file %s: %v\n", filePath, err)
	}
	return true
}
