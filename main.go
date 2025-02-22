package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"net/http"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.design/x/hotkey"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"

	"github.com/nictuku/ooosplits/speedrun"
)

const (
	windowWidth   = 400
	windowHeight  = 400
	eventDuration = time.Second
	dbPath        = "speedrun.db"

	nameColumnWidth = 160
	diffColumnWidth = 50
	timeColumnWidth = 70
	lineSpacing     = 20
	leftPadding     = 20
)

func shortenStringToFit(s string, maxWidth int, face font.Face) string {
	w := font.MeasureString(face, s).Round()
	if w <= maxWidth {
		return s
	}

	ellipsis := "... "
	ellipsisWidth := font.MeasureString(face, ellipsis).Round()
	maxContentWidth := maxWidth - ellipsisWidth

	truncated := s
	for font.MeasureString(face, truncated).Round() > maxContentWidth && len(truncated) > 0 {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + ellipsis
}

type Game struct {
	lastEvent  string
	eventTime  time.Time
	runManager *speedrun.RunManager
	isFinished bool
}

func (g *Game) Update() error {
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	bgColor := color.RGBA{0, 0, 0, 255}
	screen.Fill(bgColor)

	fontFace := basicfont.Face7x13
	white := color.RGBA{255, 255, 255, 255}
	green := color.RGBA{0, 255, 0, 255}
	gold := color.RGBA{255, 215, 0, 255}
	red := color.RGBA{255, 0, 0, 255}
	gray := color.RGBA{200, 200, 200, 255}

	title := g.runManager.GetTitle()
	category := g.runManager.GetCategory()
	completedRuns := g.runManager.GetCompletedRuns()
	attempts := g.runManager.GetAttempts()
	splitNames := g.runManager.GetSplitNames()
	currentSplitIndex := g.runManager.GetCurrentSplit()
	splits := g.runManager.GetCurrentSplits()
	pb := g.runManager.GetPersonalBest()

	pos := (windowWidth - len(title)*7) / 2
	text.Draw(screen, title, fontFace, pos, 20, white)
	text.Draw(screen, category, fontFace,
		(windowWidth-len(category)*7)/2, 40, white)
	attemptText := fmt.Sprintf("%d/%d", completedRuns, attempts)
	text.Draw(screen, attemptText, fontFace,
		(windowWidth-len(attemptText)*7)/2, 60, white)

	lineXName := leftPadding
	lineXDiffPB := lineXName + nameColumnWidth + 10
	lineXGold := lineXDiffPB + diffColumnWidth + 10
	lineXTime := lineXGold + diffColumnWidth + 10

	yPos := 80
	text.Draw(screen, "Split", fontFace, lineXName, yPos, white)
	text.Draw(screen, "vs PB", fontFace, lineXDiffPB, yPos, white)
	text.Draw(screen, "vs Gold", fontFace, lineXGold, yPos, white)
	text.Draw(screen, "Time", fontFace, lineXTime, yPos, white)

	yPos = 100

	for i, splitName := range splitNames {
		displayName := shortenStringToFit(splitName, nameColumnWidth, fontFace)

		var segmentTime time.Duration
		var cumulativeTime time.Duration
		var pbSegmentTime time.Duration
		var goldSegmentTime time.Duration
		var diffPBStr, diffGoldStr string
		var diffPBColor, diffGoldColor color.Color = white, white

		isSplitDone := (i < len(splits))

		if pb != nil && i < len(pb.Splits) {
			pbSegmentTime = pb.Splits[i].Duration
			goldSegmentTime = pb.Splits[i].BestSegment
		}
		var pbCumulativeTime time.Duration

		// Always compute the PB cumulative time if available.
		if pb != nil && i < len(pb.Splits) {
			for j := 0; j <= i; j++ {
				pbCumulativeTime += pb.Splits[j].Duration
			}
		}

		if isSplitDone {
			segmentTime = splits[i]

			for j := 0; j <= i; j++ {
				cumulativeTime += splits[j]
			}

			if pbSegmentTime > 0 {
				diffPB := cumulativeTime - pbCumulativeTime
				if diffPB < 0 {
					diffPBStr = fmt.Sprintf("-%s", formatDuration(-diffPB))
					diffPBColor = green
				} else if diffPB > 0 {
					diffPBStr = fmt.Sprintf("+%s", formatDuration(diffPB))
					diffPBColor = red
				} else {
					diffPBStr = "±0.00"
					diffPBColor = white
				}

			}

			if goldSegmentTime > 0 {
				diffGold := segmentTime - goldSegmentTime
				if diffGold < 0 {
					diffGoldStr = fmt.Sprintf("-%s", formatDuration(-diffGold))
					diffGoldColor = gold
				} else if diffGold > 0 {
					diffGoldStr = fmt.Sprintf("+%s", formatDuration(diffGold))
					diffGoldColor = red
				} else {
					diffGoldStr = "±0.00"
					diffGoldColor = white
				}
			}
		}

		if i == currentSplitIndex && !g.isFinished && g.runManager.IsRunning() {
			text.Draw(screen, displayName, fontFace, lineXName, yPos, white)
			if pbCumulativeTime > 0 {
				text.Draw(screen, formatDuration(pbCumulativeTime), fontFace, lineXTime, yPos, gray)
			}
		} else if isSplitDone {
			text.Draw(screen, displayName, fontFace, lineXName, yPos, white)
			text.Draw(screen, diffPBStr, fontFace, lineXDiffPB, yPos, diffPBColor)
			text.Draw(screen, diffGoldStr, fontFace, lineXGold, yPos, diffGoldColor)
			text.Draw(screen, formatDuration(cumulativeTime), fontFace, lineXTime, yPos, white)
		} else {
			text.Draw(screen, displayName, fontFace, lineXName, yPos, gray)
			if pbCumulativeTime > 0 {
				text.Draw(screen, formatDuration(pbCumulativeTime), fontFace, lineXTime, yPos, gray)
			}
		}

		yPos += lineSpacing
	}

	var displayTime string
	displayTime = formatDurationMicro(g.runManager.GetCurrentTime())

	scale := 3
	originalMask := basicfont.Face7x13.Mask
	bounds := originalMask.Bounds()
	newMask := ebiten.NewImage(bounds.Dx()*scale, bounds.Dy()*scale)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := originalMask.At(x, y).RGBA()
			if a > 0 {
				for sy := 0; sy < scale; sy++ {
					for sx := 0; sx < scale; sx++ {
						newMask.Set(
							(x-bounds.Min.X)*scale+sx,
							(y-bounds.Min.Y)*scale+sy,
							color.White,
						)
					}
				}
			}
		}
	}

	bigFontFace := &basicfont.Face{
		Advance: basicfont.Face7x13.Advance * scale,
		Width:   basicfont.Face7x13.Width * scale,
		Height:  basicfont.Face7x13.Height * scale,
		Ascent:  basicfont.Face7x13.Ascent * scale,
		Descent: basicfont.Face7x13.Descent * scale,
		Left:    basicfont.Face7x13.Left * scale,
		Mask:    newMask,
		Ranges:  basicfont.Face7x13.Ranges,
	}

	textWidth := font.MeasureString(bigFontFace, displayTime)
	x := (windowWidth - textWidth.Round()) / 2
	text.Draw(screen, displayTime, bigFontFace, x, 300, green)

	attributionText := "OooSplits by OopsKapootz"
	attributionFontFace := basicfont.Face7x13
	attributionWidth := font.MeasureString(attributionFontFace, attributionText).Round()
	attributionX := (windowWidth - attributionWidth) / 2
	attributionY := windowHeight - 15
	attributionColor := color.RGBA{150, 150, 150, 255}
	text.Draw(screen, attributionText, attributionFontFace, attributionX, attributionY, attributionColor)

	if time.Since(g.eventTime) < eventDuration {
		text.Draw(screen, g.lastEvent, fontFace, 500, 50, green)
	}
}

func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	centiseconds := int(d.Milliseconds()%1000) / 10

	if minutes > 0 {
		return fmt.Sprintf("%d:%02d.%02d", minutes, seconds, centiseconds)
	}
	return fmt.Sprintf("%d.%02d", seconds, centiseconds)
}

func formatDurationMicro(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	centiseconds := int(d.Milliseconds()%1000) / 10

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, seconds, centiseconds)
	}
	return fmt.Sprintf("%02d:%02d.%02d", minutes, seconds, centiseconds)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return windowWidth, windowHeight
}

func main() {
	var importFile string
	flag.StringVar(&importFile, "import", "", "Import configuration from JSON file")
	flag.Parse()

	log.Println("Starting pprof server on localhost:6060")
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	runManager, err := speedrun.NewRunManager(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize run manager: %v", err)
	}
	defer runManager.Close()

	if importFile != "" {
		log.Printf("Importing configuration from %s", importFile)
		if err := runManager.ImportFromJSON(importFile); err != nil {
			log.Fatalf("Failed to import configuration: %v", err)
		}
		log.Printf("Successfully imported configuration")
	}

	game := &Game{
		runManager: runManager,
		isFinished: false,
	}

	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle("Speedrun Timer")
	ebiten.SetTPS(60)

	go registerHotkeys(game)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

func registerHotkeys(g *Game) {
	hkSplit := hotkey.New([]hotkey.Modifier{}, hotkey.Key(0x53))
	hkReset := hotkey.New([]hotkey.Modifier{}, hotkey.Key(0x55))
	hkUndo := hotkey.New([]hotkey.Modifier{}, hotkey.Key(0x5B))

	if err := hkUndo.Register(); err != nil {
		log.Printf("Failed to register Undo hotkey: %v", err)
	}
	if err := hkReset.Register(); err != nil {
		log.Printf("Failed to register Reset hotkey: %v", err)
	}
	if err := hkSplit.Register(); err != nil {
		log.Printf("Failed to register Split hotkey: %v", err)
	}

	for {
		select {
		case <-hkSplit.Keydown():
			if g.isFinished {
				continue
			}
			if !g.runManager.IsRunning() {
				g.runManager.StartRun()
				g.lastEvent = "Started"
			} else {
				isFinished, err := g.runManager.Split()
				if err != nil {
					log.Printf("Error recording split: %v", err)
				}
				if isFinished {
					g.isFinished = true
					g.lastEvent = "Finished"
				} else {
					g.lastEvent = "Split"
				}
			}
			g.eventTime = time.Now()
			log.Println("Split triggered")

		case <-hkUndo.Keydown():
			if !g.isFinished && g.runManager.IsRunning() {
				if err := g.runManager.UndoSplit(); err != nil {
					log.Printf("Error undoing split: %v", err)
				}
				g.lastEvent = "Undo"
				g.eventTime = time.Now()
				log.Println("Undo triggered")
			}

		case <-hkReset.Keydown():
			if g.isFinished && g.runManager.IsBetterThanPB() {
				err := g.runManager.SaveAsPB()
				if err != nil {
					log.Printf("Error saving PB: %v", err)
				}
			}
			if err := g.runManager.ResetRun(); err != nil {
				log.Printf("Error resetting run: %v", err)
			}
			g.isFinished = false
			g.lastEvent = "Reset"
			g.eventTime = time.Now()
			log.Println("Reset triggered")
		}
	}
}
