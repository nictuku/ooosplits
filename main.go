package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.design/x/hotkey"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"

	"github.com/nictuku/oosplits/speedrun"
)

const (
	windowWidth   = 400
	windowHeight  = 400
	eventDuration = time.Second
	dbPath        = "speedrun.db"
)

type Game struct {
	lastEvent  string
	eventTime  time.Time
	runManager *speedrun.RunManager
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
	orange := color.RGBA{255, 165, 0, 255}

	title := g.runManager.GetTitle()
	category := g.runManager.GetCategory()
	completedRuns := g.runManager.GetCompletedRuns()
	attempts := g.runManager.GetAttempts()
	splitNames := g.runManager.GetSplitNames()
	currentSplit := g.runManager.GetCurrentSplit()
	isRunning := g.runManager.IsRunning()
	splits := g.runManager.GetCurrentSplits()
	pb := g.runManager.GetPersonalBest()

	text.Draw(screen, title, fontFace, 220, 20, white)
	text.Draw(screen, category, fontFace, 270, 40, white)

	attemptText := fmt.Sprintf("%d/%d", completedRuns, attempts)
	text.Draw(screen, attemptText, fontFace, 270, 60, white)

	maxTimeWidth := 0
	for i := range splitNames {
		splitTimeStr := "-"
		if i < len(splits) {
			splitTimeStr = formatDuration(splits[i])
		} else if i == currentSplit && isRunning {
			splitTimeStr = formatDuration(g.runManager.GetCurrentSplitTime())
		} else if pb != nil && i < len(pb.Splits) {
			splitTimeStr = "-"
		}

		timeWidth := font.MeasureString(fontFace, splitTimeStr).Round()
		if timeWidth > maxTimeWidth {
			maxTimeWidth = timeWidth
		}
	}

	yPos := 100
	for i, splitName := range splitNames {
		splitTimeStr := "-"
		diffStr := ""
		diffColor := white

		if i < len(splits) {
			splitTime := splits[i]
			splitTimeStr = formatDuration(splitTime)

			if pb != nil && i < len(pb.Splits) {
				pbSplitTime := pb.Splits[i].Duration
				timeDiff := splitTime - pbSplitTime

				if timeDiff < 0 {
					diffStr = fmt.Sprintf(" (-%s)", formatDuration(-timeDiff))
					diffColor = green
				} else if timeDiff > 0 {
					diffStr = fmt.Sprintf(" (+%s)", formatDuration(timeDiff))
					diffColor = orange
				}
			}
		} else if i == currentSplit && isRunning {
			currentSplitTime := g.runManager.GetCurrentSplitTime()
			splitTimeStr = formatDuration(currentSplitTime)

			if pb != nil && i < len(pb.Splits) {
				pbSplitTime := pb.Splits[i].Duration
				timeDiff := currentSplitTime - pbSplitTime

				if timeDiff < 0 {
					diffStr = fmt.Sprintf(" (-%s)", formatDuration(-timeDiff))
					diffColor = green
				} else if timeDiff > 0 {
					diffStr = fmt.Sprintf(" (+%s)", formatDuration(timeDiff))
					diffColor = orange
				}
			}
		} else if pb != nil && i < len(pb.Splits) {
			splitTimeStr = "-"
		}

		lineX := 20
		lineY := yPos

		timeX := lineX + font.MeasureString(fontFace, "                    ").Round()

		text.Draw(screen, splitName, fontFace, lineX, lineY, white)
		text.Draw(screen, splitTimeStr, fontFace, timeX, lineY, white)

		if diffStr != "" {
			const gap = 12
			diffX := timeX + maxTimeWidth + gap
			text.Draw(screen, diffStr, fontFace, diffX, lineY, diffColor)
		}

		yPos += 20
	}

	displayTime := formatDurationMicro(g.runManager.GetCurrentTime())

	scale := 3
	originalMask := basicfont.Face7x13.Mask
	bounds := originalMask.Bounds()
	newMask := ebiten.NewImage(bounds.Dx()*scale, bounds.Dy()*scale)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if _, _, _, a := originalMask.At(x, y).RGBA(); a > 0 {
				for sy := 0; sy < scale; sy++ {
					for sx := 0; sx < scale; sx++ {
						newMask.Set((x-bounds.Min.X)*scale+sx, (y-bounds.Min.Y)*scale+sy, color.White)
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

	if time.Since(g.eventTime) < eventDuration {
		text.Draw(screen, g.lastEvent, fontFace, 500, 50, green)
	}
}

func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	milliseconds := int(d.Milliseconds()) % 1000 / 10

	if minutes > 0 {
		return fmt.Sprintf("%d:%02d.%02d", minutes, seconds, milliseconds)
	}
	return fmt.Sprintf("%d.%02d", seconds, milliseconds)
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
			if !g.runManager.IsRunning() {
				g.runManager.StartRun()
				g.lastEvent = "Started"
			} else {
				isFinished, err := g.runManager.Split()
				if err != nil {
					log.Printf("Error recording split: %v", err)
				}

				if isFinished {
					g.lastEvent = "Finished"
				} else {
					g.lastEvent = "Split"
				}
			}
			g.eventTime = time.Now()
			log.Println("Split triggered")

		case <-hkUndo.Keydown():
			if err := g.runManager.UndoSplit(); err != nil {
				log.Printf("Error undoing split: %v", err)
			}
			g.lastEvent = "Undo"
			g.eventTime = time.Now()
			log.Println("Undo triggered")

		case <-hkReset.Keydown():
			if err := g.runManager.ResetRun(); err != nil {
				log.Printf("Error resetting run: %v", err)
			}
			g.lastEvent = "Reset"
			g.eventTime = time.Now()
			log.Println("Reset triggered")
		}
	}
}
