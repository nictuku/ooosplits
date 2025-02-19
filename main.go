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
	windowWidth   = 600
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

	// Get data from run manager
	title := g.runManager.GetTitle()
	category := g.runManager.GetCategory()
	completedRuns := g.runManager.GetCompletedRuns()
	attempts := g.runManager.GetAttempts()
	splitNames := g.runManager.GetSplitNames()
	currentSplit := g.runManager.GetCurrentSplit()
	isRunning := g.runManager.IsRunning()
	splits := g.runManager.GetCurrentSplits()
	pb := g.runManager.GetPersonalBest()

	// Draw title and category
	text.Draw(screen, title, fontFace, 220, 20, white)
	text.Draw(screen, category, fontFace, 270, 40, white)

	// Draw attempts
	attemptText := fmt.Sprintf("%d/%d", completedRuns, attempts)
	text.Draw(screen, attemptText, fontFace, 270, 60, white)

	// Draw splits
	yPos := 100
	var cumulativeTime time.Duration
	var pbCumulativeTime time.Duration

	for i, splitName := range splitNames {
		splitTimeStr := "-"
		totalTimeStr := "-"
		diffStr := ""
		diffColor := white

		if i < len(splits) {
			// This split is completed
			splitTime := splits[i]
			cumulativeTime += splitTime
			splitTimeStr = formatDuration(splitTime)
			totalTimeStr = formatDuration(cumulativeTime)

			// Compare with PB if available
			if pb != nil && i < len(pb.Splits) {
				pbSplitTime := pb.Splits[i].Duration
				pbCumulativeTime += pbSplitTime
				timeDiff := splitTime - pbSplitTime

				if timeDiff < 0 {
					// Faster than PB
					diffStr = fmt.Sprintf(" (-%s)", formatDuration(-timeDiff))
					diffColor = green
				} else if timeDiff > 0 {
					// Slower than PB
					diffStr = fmt.Sprintf(" (+%s)", formatDuration(timeDiff))
					diffColor = orange
				}
			}
		} else if i == currentSplit && isRunning {
			// Current active split
			currentSplitTime := g.runManager.GetCurrentSplitTime()
			cumulativeTime += currentSplitTime
			splitTimeStr = formatDuration(currentSplitTime)
			totalTimeStr = formatDuration(cumulativeTime)

			// For current split, compare against PB in real-time
			if pb != nil && i < len(pb.Splits) {
				pbSplitTime := pb.Splits[i].Duration
				pbCumulativeTime += pbSplitTime
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
			// Show upcoming PB splits
			pbSplitTime := pb.Splits[i].Duration
			pbCumulativeTime += pbSplitTime
			splitTimeStr = "-"
			totalTimeStr = "-"
		}

		// Draw split name and time
		splitLine := fmt.Sprintf("%-25s %10s%s %10s", splitName, splitTimeStr, "", totalTimeStr)
		text.Draw(screen, splitLine, fontFace, 50, yPos, white)

		// Draw difference separately to use different color
		if diffStr != "" {
			text.Draw(screen, diffStr, fontFace, 50+25*7+10, yPos, diffColor)
		}

		yPos += 20
	}

	// Create big timer display value
	displayTime := formatDurationMicro(g.runManager.GetCurrentTime())

	// Create scaled font mask
	scale := 3
	originalMask := basicfont.Face7x13.Mask
	bounds := originalMask.Bounds()
	newMask := ebiten.NewImage(bounds.Dx()*scale, bounds.Dy()*scale)

	// Scale up the mask image
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

	// Create scaled font
	bigFontFace := &basicfont.Face{
		Advance: basicfont.Face7x13.Advance * scale,
		Width:   basicfont.Face7x13.Width * scale,
		Height:  basicfont.Face7x13.Height * scale,
		Ascent:  basicfont.Face7x13.Ascent * scale,
		Descent: basicfont.Face7x13.Descent * scale,
		Left:    basicfont.Face7x13.Left * scale,
		Mask:    newMask,
		Ranges:  basicfont.Face7x13.Ranges, // Ranges stay the same
	}

	// Draw the big timer centered
	textWidth := font.MeasureString(bigFontFace, displayTime)
	x := (windowWidth - textWidth.Round()) / 2
	text.Draw(screen, displayTime, bigFontFace, x, 300, green)

	// Draw event text if needed
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
	centiseconds := int(d.Milliseconds()%1000) / 10 // Convert to centiseconds

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, seconds, centiseconds)
	}
	return fmt.Sprintf("%02d:%02d.%02d", minutes, seconds, centiseconds)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return windowWidth, windowHeight
}

func main() {
	// Parse command line arguments
	var importFile string
	flag.StringVar(&importFile, "import", "", "Import configuration from JSON file")
	flag.Parse()

	// Initialize run manager
	runManager, err := speedrun.NewRunManager(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize run manager: %v", err)
	}
	defer runManager.Close()

	// Import JSON configuration if specified
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

	// Register hotkeys
	go registerHotkeys(game)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

func registerHotkeys(g *Game) {
	hkSplit := hotkey.New([]hotkey.Modifier{}, hotkey.Key(0x53)) // NumPad1
	hkReset := hotkey.New([]hotkey.Modifier{}, hotkey.Key(0x55)) // NumPad3
	hkUndo := hotkey.New([]hotkey.Modifier{}, hotkey.Key(0x5B))  // NumPad8

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
				// Start the timer
				g.runManager.StartRun()
				g.lastEvent = "Started"
			} else {
				// Record split time
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
