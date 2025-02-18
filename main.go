package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io/ioutil"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.design/x/hotkey"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

const (
	windowWidth   = 600
	windowHeight  = 400
	eventDuration = time.Second
)

type Split struct {
	Time string `json:"time"`
}

type Config struct {
	Title        string   `json:"title"`
	Category     string   `json:"category"`
	Attempts     int      `json:"attempts"`
	Completed    int      `json:"completed"`
	SplitNames   []string `json:"split_names"`
	PersonalBest struct {
		Attempt int     `json:"attempt"`
		Splits  []Split `json:"splits"`
	} `json:"personal_best"`
}

type Game struct {
	lastEvent      string
	eventTime      time.Time
	config         Config
	startTime      time.Time
	splitStartTime time.Time
	isRunning      bool
	currentSplit   int
	splits         []time.Duration
	completed      bool
}

func loadConfig(filename string) (Config, error) {
	var config Config
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return config, fmt.Errorf("error reading config file: %v", err)
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, fmt.Errorf("error parsing config file: %v", err)
	}

	return config, nil
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

	// Draw title and category
	text.Draw(screen, g.config.Title, fontFace, 220, 20, white)
	text.Draw(screen, g.config.Category, fontFace, 270, 40, white)

	// Draw attempts
	attemptText := fmt.Sprintf("%d/%d", g.config.Completed, g.config.Attempts)
	text.Draw(screen, attemptText, fontFace, 270, 60, white)

	// Draw splits
	yPos := 100
	var totalTime time.Duration
	for i, splitName := range g.config.SplitNames {
		splitTimeStr := "-"
		totalTimeStr := "-"
		
		if i < len(g.splits) {
			splitTime := g.splits[i]
			totalTime += splitTime
			splitTimeStr = formatDuration(splitTime)
			totalTimeStr = formatDuration(totalTime)
		} else if i == g.currentSplit && g.isRunning {
			currentSplitTime := time.Since(g.splitStartTime)
			totalTime += currentSplitTime
			splitTimeStr = formatDuration(currentSplitTime)
			totalTimeStr = formatDuration(totalTime)
		}

		splitLine := fmt.Sprintf("%-25s %10s %10s", splitName, splitTimeStr, totalTimeStr)
		text.Draw(screen, splitLine, fontFace, 50, yPos, white)
		yPos += 20
	}

	// Create big timer display value
	var displayTime string
	if !g.isRunning && len(g.splits) == 0 {
		displayTime = "0:00:00.00"
	} else if g.completed {
		var total time.Duration
		for _, split := range g.splits {
			total += split
		}
		displayTime = formatDurationMicro(total)
	} else if g.isRunning {
		currentTime := time.Since(g.startTime)
		displayTime = formatDurationMicro(currentTime)
	}

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
	centiseconds := int(d.Milliseconds() % 1000) / 10 // Convert to centiseconds

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, seconds, centiseconds)
	}
	return fmt.Sprintf("%02d:%02d.%02d", minutes, seconds, centiseconds)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return windowWidth, windowHeight
}

func main() {
	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatal(err)
	}

	game := &Game{
		config: config,
		splits: make([]time.Duration, 0, len(config.SplitNames)),
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
			if !g.isRunning {
				// Start the timer
				g.isRunning = true
				g.startTime = time.Now()
				g.splitStartTime = g.startTime
				g.currentSplit = 0
				g.lastEvent = "Started"
			} else if g.currentSplit < len(g.config.SplitNames) {
				// Record split time
				splitDuration := time.Since(g.splitStartTime)
				g.splits = append(g.splits, splitDuration)
				
				if g.currentSplit == len(g.config.SplitNames)-1 {
					// This was the last split
					g.isRunning = false
					g.completed = true
					g.lastEvent = "Finished"
				} else {
					// Start next split
					g.currentSplit++
					g.splitStartTime = time.Now()
					g.lastEvent = "Split"
				}
			}
			g.eventTime = time.Now()
			log.Println("Split triggered")

		case <-hkUndo.Keydown():
			if g.isRunning && g.currentSplit > 0 {
				// Remove last split and go back
				g.splits = g.splits[:len(g.splits)-1]
				g.currentSplit--
				g.splitStartTime = time.Now()
				g.completed = false
			}
			g.lastEvent = "Undo"
			g.eventTime = time.Now()
			log.Println("Undo triggered")

		case <-hkReset.Keydown():
			// Reset everything
			g.isRunning = false
			g.currentSplit = 0
			g.splits = make([]time.Duration, 0, len(g.config.SplitNames))
			g.completed = false
			g.lastEvent = "Reset"
			g.eventTime = time.Now()
			log.Println("Reset triggered")
		}
	}
}