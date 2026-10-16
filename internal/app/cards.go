package app

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

//go:embed assets/fonts/DejaVuSans-Bold.ttf
var fontBoldBytes []byte

//go:embed assets/fonts/DejaVuSans.ttf
var fontRegularBytes []byte

var (
	parsedFontBold    *truetype.Font
	parsedFontRegular *truetype.Font
	fontsInitOnce     sync.Once
	fontsInitErr      error

	faceCacheMu sync.Mutex
	faceCache   = make(map[string]font.Face)

	heroImageCache   sync.Map // int -> image.Image
	avatarImageCache sync.Map // string -> image.Image
)

func initFonts() error {
	fontsInitOnce.Do(func() {
		var err error
		parsedFontBold, err = truetype.Parse(fontBoldBytes)
		if err != nil {
			fontsInitErr = fmt.Errorf("parse bold font: %w", err)
			return
		}
		parsedFontRegular, err = truetype.Parse(fontRegularBytes)
		if err != nil {
			fontsInitErr = fmt.Errorf("parse regular font: %w", err)
			return
		}
	})
	return fontsInitErr
}

func getFontFace(bold bool, size float64) (font.Face, error) {
	if err := initFonts(); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%v_%.1f", bold, size)

	faceCacheMu.Lock()
	defer faceCacheMu.Unlock()
	if f, ok := faceCache[key]; ok {
		return f, nil
	}

	targetFont := parsedFontRegular
	if bold {
		targetFont = parsedFontBold
	}

	face := truetype.NewFace(targetFont, &truetype.Options{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	faceCache[key] = face
	return face, nil
}

func setRGBAColor(dc *gg.Context, c color.RGBA) {
	dc.SetRGBA255(int(c.R), int(c.G), int(c.B), int(c.A))
}

func setRGBColor(dc *gg.Context, c color.RGBA) {
	dc.SetRGB255(int(c.R), int(c.G), int(c.B))
}

type ratingEntryData struct {
	Name    string
	Winrate float64
	Games   int
}

type matchCardInput struct {
	PlayerName   string
	HeroID       int
	HeroName     string
	HeroShort    string
	AvatarURL    string
	IsWin        bool
	Kills        int
	Deaths       int
	Assists      int
	Duration     int
	HeroDamage   int
	NetWorth     int
	Items        []string
	StreakCount  int
	StreakIsWin  bool
	MatchID      int64
	StartTime    int64
	RuinAnalysis RuinAnalysis
}

type ratingCardInput struct {
	Rank    int
	Name    string
	Winrate float64
	Games   int
}

func renderMatchCard(in matchCardInput) ([]byte, error) {
	if err := initFonts(); err != nil {
		return nil, err
	}

	w, h := 860, 460
	dc := gg.NewContext(w, h)

	// Background color
	dc.SetRGB255(16, 18, 24)
	dc.Clear()

	// Glow gradient in top-right corner
	glowColor := color.RGBA{R: 0, G: 180, B: 90, A: 45}
	if !in.IsWin {
		glowColor = color.RGBA{R: 200, G: 30, B: 30, A: 45}
	}
	for r := 240.0; r > 0; r -= 20 {
		alpha := float64(glowColor.A) * (1.0 - r/240.0) / 255.0
		dc.SetRGBA(float64(glowColor.R)/255.0, float64(glowColor.G)/255.0, float64(glowColor.B)/255.0, alpha)
		dc.DrawEllipse(float64(w)-160, 10, r, r)
		dc.Fill()
	}

	// Outer rounded border
	dc.DrawRoundedRectangle(1, 1, float64(w)-2, float64(h)-2, 22)
	dc.SetRGB255(42, 48, 64)
	dc.SetLineWidth(2)
	dc.Stroke()

	// Player Avatar
	avatarSize := 54
	avX, avY := 36.0, 30.0
	avatarImg := getAvatarImage(in.AvatarURL)
	if avatarImg != nil {
		circleAv := makeCircleAvatar(avatarImg, avatarSize)
		dc.DrawImage(circleAv, int(avX), int(avY))
	} else {
		dc.DrawCircle(avX+float64(avatarSize)/2, avY+float64(avatarSize)/2, float64(avatarSize)/2)
		dc.SetRGB255(48, 54, 72)
		dc.Fill()
		if len(in.PlayerName) > 0 {
			f, _ := getFontFace(true, 22)
			dc.SetFontFace(f)
			dc.SetRGB255(220, 225, 235)
			firstChar := string([]rune(in.PlayerName)[0])
			dc.DrawStringAnchored(firstChar, avX+float64(avatarSize)/2, avY+float64(avatarSize)/2, 0.5, 0.5)
		}
	}
	dc.DrawCircle(avX+float64(avatarSize)/2, avY+float64(avatarSize)/2, float64(avatarSize)/2)
	dc.SetRGB255(68, 78, 102)
	dc.SetLineWidth(2)
	dc.Stroke()

	// Player Name & Subtitle
	nameFace, _ := getFontFace(true, 26)
	dc.SetFontFace(nameFace)
	dc.SetRGB255(245, 248, 252)
	maxNameLen := 18
	if in.RuinAnalysis.IsRuiner || in.RuinAnalysis.IsSuspect {
		maxNameLen = 13
	}
	dc.DrawString(trimTo(fallbackName(in.PlayerName), maxNameLen), 104, 52)

	subFace, _ := getFontFace(false, 14)
	dc.SetFontFace(subFace)
	dc.SetRGB255(125, 138, 162)
	dc.DrawString("Dota 2 Match Notification • easyKatka", 104, 76)

	// Result Badge (ПОБЕДА / ПОРАЖЕНИЕ)
	badgeW, badgeH := 160.0, 42.0
	badgeX := float64(w) - badgeW - 36.0
	badgeY := 32.0

	dc.DrawRoundedRectangle(badgeX, badgeY, badgeW, badgeH, 10)
	if in.IsWin {
		dc.SetRGBA255(10, 54, 30, 240)
		dc.FillPreserve()
		dc.SetRGB255(0, 230, 118)
		dc.SetLineWidth(2)
		dc.Stroke()

		badgeFace, _ := getFontFace(true, 20)
		dc.SetFontFace(badgeFace)
		dc.SetRGB255(0, 230, 118)
		dc.DrawStringAnchored("ПОБЕДА", badgeX+badgeW/2, badgeY+badgeH/2-1, 0.5, 0.5)
	} else {
		dc.SetRGBA255(56, 16, 20, 240)
		dc.FillPreserve()
		dc.SetRGB255(255, 68, 68)
		dc.SetLineWidth(2)
		dc.Stroke()

		badgeFace, _ := getFontFace(true, 20)
		dc.SetFontFace(badgeFace)
		dc.SetRGB255(255, 68, 68)
		dc.DrawStringAnchored("ПОРАЖЕНИЕ", badgeX+badgeW/2, badgeY+badgeH/2-1, 0.5, 0.5)
	}

	// Ruin Badge (if Ruiner or Suspected)
	if in.RuinAnalysis.IsRuiner {
		ruinW, ruinH := 140.0, 42.0
		ruinX := badgeX - ruinW - 12.0
		dc.DrawRoundedRectangle(ruinX, badgeY, ruinW, ruinH, 10)
		dc.SetRGBA255(65, 12, 18, 245)
		dc.FillPreserve()
		dc.SetRGB255(255, 45, 45)
		dc.SetLineWidth(2)
		dc.Stroke()

		rFace, _ := getFontFace(true, 18)
		dc.SetFontFace(rFace)
		dc.SetRGB255(255, 55, 55)
		dc.DrawStringAnchored("РУИНЕР", ruinX+ruinW/2, badgeY+ruinH/2-1, 0.5, 0.5)
	} else if in.RuinAnalysis.IsSuspect {
		ruinW, ruinH := 160.0, 42.0
		ruinX := badgeX - ruinW - 12.0
		dc.DrawRoundedRectangle(ruinX, badgeY, ruinW, ruinH, 10)
		dc.SetRGBA255(55, 35, 10, 245)
		dc.FillPreserve()
		dc.SetRGB255(255, 165, 0)
		dc.SetLineWidth(2)
		dc.Stroke()

		rFace, _ := getFontFace(true, 17)
		dc.SetFontFace(rFace)
		dc.SetRGB255(255, 175, 0)
		dc.DrawStringAnchored("ПОДОЗРЕНИЕ", ruinX+ruinW/2, badgeY+ruinH/2-1, 0.5, 0.5)
	}

	// Divider
	dc.DrawLine(36, 100, float64(w)-36, 100)
	dc.SetRGB255(32, 38, 52)
	dc.SetLineWidth(1)
	dc.Stroke()

	// Hero Card (Left center)
	heroCardX, heroCardY := 36.0, 120.0
	heroCardW, heroCardH := 246.0, 215.0
	dc.DrawRoundedRectangle(heroCardX, heroCardY, heroCardW, heroCardH, 16)
	dc.SetRGB255(23, 27, 36)
	dc.FillPreserve()
	dc.SetRGB255(40, 46, 62)
	dc.SetLineWidth(1)
	dc.Stroke()

	// Draw Hero Image
	heroImg := getHeroImage(in.HeroID, in.HeroShort)
	if heroImg != nil {
		hW, hH := 222, 125
		resizedHero := resizeImage(heroImg, hW, hH)
		roundedHero := makeRoundedImage(resizedHero, 12)
		dc.DrawImage(roundedHero, int(heroCardX)+12, int(heroCardY)+12)
	}

	heroTitleFace, _ := getFontFace(true, 23)
	dc.SetFontFace(heroTitleFace)
	dc.SetRGB255(255, 255, 255)
	heroNameDisplay := in.HeroName
	if heroNameDisplay == "" {
		heroNameDisplay = fmt.Sprintf("Hero #%d", in.HeroID)
	}
	dc.DrawString(trimTo(heroNameDisplay, 16), heroCardX+16, heroCardY+168)

	dc.SetFontFace(subFace)
	dc.SetRGB255(115, 128, 150)
	dc.DrawString("Герой матча", heroCardX+16, heroCardY+194)

	// 4 Stats Cards
	statsX := 310.0
	type statItem struct {
		Title string
		Value string
		Color color.RGBA
	}
	stats := []statItem{
		{
			Title: "K / D / A",
			Value: fmt.Sprintf("%d / %d / %d", in.Kills, in.Deaths, in.Assists),
			Color: color.RGBA{R: 255, G: 255, B: 255, A: 255},
		},
		{
			Title: "ДЛИТЕЛЬНОСТЬ",
			Value: formatDuration(in.Duration),
			Color: color.RGBA{R: 220, G: 225, B: 235, A: 255},
		},
		{
			Title: "УРОН ПО ГЕРОЯМ",
			Value: formatNumberK(in.HeroDamage),
			Color: color.RGBA{R: 255, G: 185, B: 0, A: 255},
		},
		{
			Title: "NET WORTH",
			Value: formatNumberK(in.NetWorth),
			Color: color.RGBA{R: 64, G: 196, B: 255, A: 255},
		},
	}

	lblFace, _ := getFontFace(true, 13)
	valFace, _ := getFontFace(true, 28)

	for i, s := range stats {
		col := i % 2
		row := i / 2
		bx := statsX + float64(col)*260.0
		by := heroCardY + float64(row)*112.0
		bw := 246.0
		bh := 98.0

		dc.DrawRoundedRectangle(bx, by, bw, bh, 14)
		dc.SetRGB255(23, 27, 36)
		dc.FillPreserve()
		dc.SetRGB255(36, 42, 56)
		dc.SetLineWidth(1)
		dc.Stroke()

		dc.SetFontFace(lblFace)
		dc.SetRGB255(125, 138, 162)
		dc.DrawString(s.Title, bx+18, by+30)

		dc.SetFontFace(valFace)
		setRGBAColor(dc, s.Color)
		dc.DrawString(s.Value, bx+18, by+72)
	}

	// Bottom Row
	itemsY := 352.0
	dc.DrawLine(36, itemsY, float64(w)-36, itemsY)
	dc.SetRGB255(32, 38, 52)
	dc.SetLineWidth(1)
	dc.Stroke()

	curX := 36.0
	tagFace, _ := getFontFace(true, 14)
	dc.SetFontFace(tagFace)

	// Streak badge
	if in.StreakCount >= 2 {
		streakText := fmt.Sprintf("Серия: %d %s", in.StreakCount, pluralize(in.StreakCount, "победа", "победы", "побед"))
		if !in.StreakIsWin {
			streakText = fmt.Sprintf("Серия: %d %s", in.StreakCount, pluralize(in.StreakCount, "поражение", "поражения", "поражений"))
		}

		stW, _ := dc.MeasureString(streakText)
		badgeW := stW + 28
		stH := 32.0
		stY := itemsY + 16

		dc.DrawRoundedRectangle(curX, stY, badgeW, stH, 8)
		if in.StreakIsWin {
			dc.SetRGB255(42, 28, 14)
			dc.FillPreserve()
			dc.SetRGB255(255, 145, 0)
			dc.SetLineWidth(1)
			dc.Stroke()
			dc.SetRGB255(255, 190, 60)
		} else {
			dc.SetRGB255(38, 18, 22)
			dc.FillPreserve()
			dc.SetRGB255(255, 80, 80)
			dc.SetLineWidth(1)
			dc.Stroke()
			dc.SetRGB255(255, 120, 120)
		}
		dc.DrawStringAnchored(streakText, curX+badgeW/2, stY+stH/2-1, 0.5, 0.5)
		curX += badgeW + 12
	}

	// Items chips
	itemFace, _ := getFontFace(false, 13)
	dc.SetFontFace(itemFace)
	for _, item := range in.Items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		itW, _ := dc.MeasureString(item)
		chipW := itW + 20
		chipH := 30.0
		chipY := itemsY + 17
		if curX+chipW > float64(w)-220 {
			break
		}
		dc.DrawRoundedRectangle(curX, chipY, chipW, chipH, 8)
		dc.SetRGB255(28, 33, 45)
		dc.FillPreserve()
		dc.SetRGB255(44, 52, 70)
		dc.SetLineWidth(1)
		dc.Stroke()

		dc.SetRGB255(185, 196, 215)
		dc.DrawStringAnchored(item, curX+chipW/2, chipY+chipH/2-1, 0.5, 0.5)
		curX += chipW + 8
	}

	// Footer date & Match ID
	dateText := ""
	if in.StartTime > 0 {
		dateText = time.Unix(in.StartTime, 0).Local().Format("02 янв, 15:04")
	}
	footerText := fmt.Sprintf("Матч #%d", in.MatchID)
	if dateText != "" {
		footerText = fmt.Sprintf("Матч #%d  •  %s", in.MatchID, dateText)
	}
	if in.RuinAnalysis.IsRuiner {
		footerText += fmt.Sprintf("  •  Ruin Score: %d/100", in.RuinAnalysis.Score)
	} else if in.RuinAnalysis.IsSuspect {
		footerText += fmt.Sprintf("  •  Ruin Score: %d/100", in.RuinAnalysis.Score)
	} else if len(in.RuinAnalysis.MatchRuiners) > 0 {
		ruinerCount := 0
		for _, r := range in.RuinAnalysis.MatchRuiners {
			if r.Analysis.IsRuiner {
				ruinerCount++
			}
		}
		if ruinerCount > 0 {
			footerText += fmt.Sprintf("  •  Руинеров в матче: %d", ruinerCount)
		} else {
			footerText += fmt.Sprintf("  •  Подозрение в матче: %d", len(in.RuinAnalysis.MatchRuiners))
		}
	}

	dc.SetFontFace(subFace)
	dc.SetRGB255(110, 122, 142)
	dc.DrawStringAnchored(footerText, float64(w)-36, itemsY+32, 1.0, 0.5)

	return encodeContextPNG(dc)
}

func encodeContextPNG(dc *gg.Context) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

func renderRatingCard(entries []ratingCardInput) ([]byte, error) {
	if err := initFonts(); err != nil {
		return nil, err
	}

	rowH := 76.0
	rowSpacing := 12.0
	startY := 120.0
	totalRows := len(entries)
	if totalRows == 0 {
		totalRows = 1
	}

	w := 860
	h := int(startY + float64(totalRows)*(rowH+rowSpacing) + 24.0)

	dc := gg.NewContext(w, h)
	dc.SetRGB255(16, 18, 24)
	dc.Clear()

	// Ambient yellow glow in header
	for r := 200.0; r > 0; r -= 20 {
		alpha := 0.15 * (1.0 - r/200.0)
		dc.SetRGBA(1.0, 0.8, 0.1, alpha)
		dc.DrawEllipse(float64(w)/2, 10, r*1.6, r*0.7)
		dc.Fill()
	}

	// Border
	dc.DrawRoundedRectangle(1, 1, float64(w)-2, float64(h)-2, 22)
	dc.SetRGB255(42, 48, 64)
	dc.SetLineWidth(2)
	dc.Stroke()

	// Header
	titleFace, _ := getFontFace(true, 26)
	dc.SetFontFace(titleFace)
	dc.SetRGB255(255, 255, 255)
	dc.DrawString("РЕЙТИНГ ИГРОКОВ ПО WINRATE", 36, 52)

	subFace, _ := getFontFace(false, 14)
	dc.SetFontFace(subFace)
	dc.SetRGB255(125, 138, 162)
	dc.DrawString("Статистика по последним 50 матчам • easyKatka Team", 36, 76)

	// Separator
	dc.DrawLine(36, 102, float64(w)-36, 102)
	dc.SetRGB255(32, 38, 52)
	dc.SetLineWidth(1)
	dc.Stroke()

	rankFace, _ := getFontFace(true, 19)
	nameFace, _ := getFontFace(true, 19)
	wrFace, _ := getFontFace(true, 21)

	for i, p := range entries {
		ry := startY + float64(i)*(rowH+rowSpacing)

		// Row Card Background
		dc.DrawRoundedRectangle(36, ry, float64(w)-72, rowH, 14)
		dc.SetRGB255(23, 27, 36)
		dc.FillPreserve()
		dc.SetRGB255(36, 42, 56)
		dc.SetLineWidth(1)
		dc.Stroke()

		// Rank pill colors
		badgeW, badgeH := 44.0, 44.0
		badgeX := 52.0
		badgeY := ry + (rowH-badgeH)/2

		var borderColor, bgColor, textColor color.RGBA
		switch p.Rank {
		case 1:
			borderColor = color.RGBA{R: 255, G: 215, B: 0, A: 255}
			bgColor = color.RGBA{R: 48, G: 40, B: 15, A: 255}
			textColor = borderColor
		case 2:
			borderColor = color.RGBA{R: 210, G: 220, B: 235, A: 255}
			bgColor = color.RGBA{R: 35, G: 40, B: 48, A: 255}
			textColor = borderColor
		case 3:
			borderColor = color.RGBA{R: 215, G: 135, B: 60, A: 255}
			bgColor = color.RGBA{R: 42, G: 30, B: 20, A: 255}
			textColor = borderColor
		default:
			borderColor = color.RGBA{R: 140, G: 152, B: 175, A: 255}
			bgColor = color.RGBA{R: 28, G: 32, B: 42, A: 255}
			textColor = borderColor
		}

		dc.DrawRoundedRectangle(badgeX, badgeY, badgeW, badgeH, 10)
		setRGBAColor(dc, bgColor)
		dc.FillPreserve()
		setRGBAColor(dc, borderColor)
		dc.SetLineWidth(2)
		dc.Stroke()

		dc.SetFontFace(rankFace)
		setRGBAColor(dc, textColor)
		dc.DrawStringAnchored(fmt.Sprintf("%d", p.Rank), badgeX+badgeW/2, badgeY+badgeH/2-1, 0.5, 0.5)

		// Player Name
		dc.SetFontFace(nameFace)
		dc.SetRGB255(245, 248, 252)
		dc.DrawString(trimTo(p.Name, 18), 116, ry+32)

		// Win/Loss subtitle
		wins := int(p.Winrate * float64(p.Games) / 100.0)
		losses := p.Games - wins
		recordText := fmt.Sprintf("%dW - %dL • Всего %d матчей", wins, losses, p.Games)
		dc.SetFontFace(subFace)
		dc.SetRGB255(125, 138, 162)
		dc.DrawString(recordText, 116, ry+56)

		// Progress bar
		barX := float64(w) - 390.0
		barY := ry + 30.0
		barW := 200.0
		barH := 16.0

		dc.DrawRoundedRectangle(barX, barY, barW, barH, 8)
		dc.SetRGB255(34, 40, 54)
		dc.Fill()

		fillW := barW * (p.Winrate / 100.0)
		if fillW < 4 && p.Winrate > 0 {
			fillW = 4
		}
		if fillW > barW {
			fillW = barW
		}

		var barColor color.RGBA
		if p.Winrate >= 50.0 {
			barColor = color.RGBA{R: 0, G: 230, B: 118, A: 255}
		} else {
			barColor = color.RGBA{R: 255, G: 95, B: 95, A: 255}
		}

		if fillW > 0 {
			dc.DrawRoundedRectangle(barX, barY, fillW, barH, 8)
			setRGBAColor(dc, barColor)
			dc.Fill()
		}

		// Winrate Text
		dc.SetFontFace(wrFace)
		setRGBAColor(dc, barColor)
		wrText := fmt.Sprintf("%.1f%%", p.Winrate)
		dc.DrawStringAnchored(wrText, float64(w)-56, ry+rowH/2-1, 1.0, 0.5)
	}

	return encodeContextPNG(dc)
}

func formatNumberK(num int) string {
	if num <= 0 {
		return "—"
	}
	if num >= 1000 {
		return fmt.Sprintf("%.1fk", float64(num)/1000.0)
	}
	return fmt.Sprintf("%d", num)
}

func resizeImage(src image.Image, targetW, targetH int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	srcB := src.Bounds()
	sw, sh := srcB.Dx(), srcB.Dy()

	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			sx := x * sw / targetW
			sy := y * sh / targetH
			dst.Set(x, y, src.At(srcB.Min.X+sx, srcB.Min.Y+sy))
		}
	}
	return dst
}

func makeRoundedImage(src image.Image, radius float64) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	dc := gg.NewContext(w, h)
	dc.DrawRoundedRectangle(0, 0, float64(w), float64(h), radius)
	dc.Clip()
	dc.DrawImage(src, 0, 0)
	return dc.Image()
}

func makeCircleAvatar(src image.Image, size int) image.Image {
	resized := resizeImage(src, size, size)
	dc := gg.NewContext(size, size)
	dc.DrawCircle(float64(size)/2, float64(size)/2, float64(size)/2)
	dc.Clip()
	dc.DrawImage(resized, 0, 0)
	return dc.Image()
}

func getHeroImage(heroID int, shortName string) image.Image {
	if val, ok := heroImageCache.Load(heroID); ok {
		img := val.(image.Image)
		if img.Bounds().Dx() >= 100 {
			return img
		}
	}

	if shortName == "" {
		shortName = getHeroShortName(heroID)
	}

	var lowResFallback image.Image

	// 1. Check local images/heroes/<shortName>.png
	if shortName != "" {
		localPath := filepath.Join("images", "heroes", shortName+".png")
		if f, err := os.Open(localPath); err == nil {
			defer f.Close()
			if img, _, err := image.Decode(f); err == nil {
				if img.Bounds().Dx() >= 100 {
					heroImageCache.Store(heroID, img)
					return img
				}
				lowResFallback = img
			}
		}
	}

	// 2. Try HD cache in /tmp
	if shortName != "" {
		cachePath := filepath.Join("/tmp", "easykatka_heroes", shortName+".png")
		if f, err := os.Open(cachePath); err == nil {
			defer f.Close()
			if img, _, err := image.Decode(f); err == nil {
				if img.Bounds().Dx() >= 100 {
					heroImageCache.Store(heroID, img)
					return img
				}
			}
		}
	}

	// 3. Download HD (256x144) from Steam CDN
	if shortName != "" {
		url := fmt.Sprintf("https://cdn.cloudflare.steamstatic.com/apps/dota2/images/dota_react/heroes/%s.png", shortName)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err == nil {
			req.Header.Set("User-Agent", "easyKatka/2.0")
			if resp, err := sharedHTTPClient.Do(req); err == nil && resp.StatusCode == 200 {
				defer resp.Body.Close()
				if img, _, err := image.Decode(resp.Body); err == nil {
					cachePath := filepath.Join("/tmp", "easykatka_heroes", shortName+".png")
					_ = os.MkdirAll(filepath.Dir(cachePath), 0755)
					_ = savePNGFile(cachePath, img)
					heroImageCache.Store(heroID, img)
					return img
				}
			}
		}
	}

	// 4. Fall back to low-res only if HD is completely unavailable
	if lowResFallback != nil {
		heroImageCache.Store(heroID, lowResFallback)
		return lowResFallback
	}

	return nil
}

func savePNGFile(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func getAvatarImage(url string) image.Image {
	if url == "" {
		return nil
	}
	if val, ok := avatarImageCache.Load(url); ok {
		return val.(image.Image)
	}

	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != 200 {
		return nil
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil
	}
	avatarImageCache.Store(url, img)
	return img
}

// Helpers for bot handlers:

func buildMatchCardFromDetails(details matchDetails, player matchDetailsPlayer, personaName string, avatarURL string, heroes map[int]string, itemNames map[int]string, streakIsWin bool, streakCount int) ([]byte, error) {
	heroName := heroes[player.HeroID]
	if heroName == "" {
		heroName = fmt.Sprintf("Hero #%d", player.HeroID)
	}

	isWin := matchWin(recentMatch{PlayerSlot: player.PlayerSlot, RadiantWin: details.RadiantWin})
	items := collectPlayerItems(player, itemNames)

	analysis := EvaluateRuin(RuinEvaluationInput{
		Duration:     details.Duration,
		Kills:        player.Kills,
		Deaths:       player.Deaths,
		Assists:      player.Assists,
		GPM:          player.GPM,
		XPM:          player.XPM,
		HeroDamage:   player.HeroDamage,
		NetWorth:     player.NetWorth,
		LeaverStatus: player.LeaverStatus,
		Pings:        player.Pings,
		Items:        items,
		IsWin:        isWin,
	})
	analysis.MatchRuiners = EvaluateMatchRuiners(details, player.AccountID, player.PlayerSlot, heroes, itemNames)

	input := matchCardInput{
		PlayerName:   fallbackName(personaName),
		HeroID:       player.HeroID,
		HeroName:     heroName,
		HeroShort:    getHeroShortName(player.HeroID),
		AvatarURL:    avatarURL,
		IsWin:        isWin,
		Kills:        player.Kills,
		Deaths:       player.Deaths,
		Assists:      player.Assists,
		Duration:     details.Duration,
		HeroDamage:   player.HeroDamage,
		NetWorth:     player.NetWorth,
		Items:        items,
		StreakCount:  streakCount,
		StreakIsWin:  streakIsWin,
		MatchID:      details.MatchID,
		StartTime:    details.StartTime,
		RuinAnalysis: analysis,
	}

	return renderMatchCard(input)
}

func buildMatchCard(accountID int64, match recentMatch, playerName string, avatarURL string, heroes map[int]string, streakIsWin bool, streakCount int) ([]byte, RuinAnalysis, error) {
	heroName := heroes[match.HeroID]
	if heroName == "" {
		heroName = fmt.Sprintf("Hero #%d", match.HeroID)
	}

	netWorth := 0
	if match.Duration > 0 && match.GoldPerMin > 0 {
		netWorth = (match.GoldPerMin * match.Duration) / 60
	}

	input := matchCardInput{
		PlayerName:  fallbackName(playerName),
		HeroID:      match.HeroID,
		HeroName:    heroName,
		HeroShort:   getHeroShortName(match.HeroID),
		AvatarURL:   avatarURL,
		IsWin:       matchWin(match),
		Kills:       match.Kills,
		Deaths:      match.Deaths,
		Assists:     match.Assists,
		Duration:    match.Duration,
		HeroDamage:  match.HeroDamage,
		NetWorth:    netWorth,
		StreakCount: streakCount,
		StreakIsWin: streakIsWin,
		MatchID:     match.MatchID,
		StartTime:   match.StartTime,
	}

	var matchedPlayer *matchDetailsPlayer
	var matchRuiners []MatchPlayerRuin
	if match.MatchID > 0 {
		if details, err := fetchMatchDetails(match.MatchID); err == nil {
			for i := range details.Players {
				if (accountID > 0 && details.Players[i].AccountID == accountID) || details.Players[i].PlayerSlot == match.PlayerSlot {
					matchedPlayer = &details.Players[i]
					break
				}
			}
			itemNames, _ := getItemNames()
			if matchedPlayer != nil {
				if matchedPlayer.HeroDamage > 0 {
					input.HeroDamage = matchedPlayer.HeroDamage
				}
				if matchedPlayer.NetWorth > 0 {
					input.NetWorth = matchedPlayer.NetWorth
				}
				if itemNames != nil {
					input.Items = collectPlayerItems(*matchedPlayer, itemNames)
				}
			}
			trackedSlot := match.PlayerSlot
			if matchedPlayer != nil {
				trackedSlot = matchedPlayer.PlayerSlot
			}
			matchRuiners = EvaluateMatchRuiners(details, accountID, trackedSlot, heroes, itemNames)
		}
	}

	ruinIn := RuinEvaluationInput{
		Duration:     match.Duration,
		Kills:        match.Kills,
		Deaths:       match.Deaths,
		Assists:      match.Assists,
		GPM:          match.GoldPerMin,
		HeroDamage:   input.HeroDamage,
		NetWorth:     input.NetWorth,
		LeaverStatus: match.LeaverStatus,
		Items:        input.Items,
		IsWin:        input.IsWin,
	}
	if matchedPlayer != nil {
		ruinIn.Pings = matchedPlayer.Pings
		if matchedPlayer.LeaverStatus > 0 {
			ruinIn.LeaverStatus = matchedPlayer.LeaverStatus
		}
		if matchedPlayer.GPM > 0 {
			ruinIn.GPM = matchedPlayer.GPM
		}
		if matchedPlayer.XPM > 0 {
			ruinIn.XPM = matchedPlayer.XPM
		}
	}
	analysis := EvaluateRuin(ruinIn)
	analysis.MatchRuiners = matchRuiners
	input.RuinAnalysis = analysis

	cardBytes, err := renderMatchCard(input)
	return cardBytes, analysis, err
}

func buildMatchCardFromRecent(match recentMatch, playerName string, avatarURL string, heroes map[int]string, streakIsWin bool, streakCount int) ([]byte, error) {
	cardBytes, _, err := buildMatchCard(0, match, playerName, avatarURL, heroes, streakIsWin, streakCount)
	return cardBytes, err
}

func buildRatingCardImage(accountIDs []int64) ([]byte, error) {
	entries := make([]ratingEntryData, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		player, err := fetchPlayerProfile(accountID)
		if err != nil {
			return nil, err
		}
		matches, err := fetchPlayerMatches(accountID, 50)
		if err != nil {
			return nil, err
		}
		winrate, games := calcWinrateWithCount(matches, 50)
		name := player.PersonaName
		if name == "" {
			name = "неизвестный"
		}
		entries = append(entries, ratingEntryData{
			Name:    name,
			Winrate: winrate,
			Games:   games,
		})
	}

	sortEntries(entries)

	ratingInputs := make([]ratingCardInput, len(entries))
	for i, e := range entries {
		ratingInputs[i] = ratingCardInput{
			Rank:    i + 1,
			Name:    e.Name,
			Winrate: e.Winrate,
			Games:   e.Games,
		}
	}

	return renderRatingCard(ratingInputs)
}

func sortEntries(entries []ratingEntryData) {
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].Winrate < entries[j].Winrate ||
				(entries[i].Winrate == entries[j].Winrate && entries[i].Games < entries[j].Games) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}
