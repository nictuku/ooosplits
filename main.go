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
	lastEvent     string
	eventTime     time.Time
	config        Config
	startTime     time.Time
	splitStartTime time.Time
	isRunning     bool
	currentSplit  int
	splits        []time.Duration
	completed     bool
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

	// Draw big timer
	var displayTime string
	if !g.isRunning && len(g.splits) == 0 {
		displayTime = "0.000"
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

	// Create bigger font - using 3x scale
	bigFontFace := &basicfont.Face{
		Advance: 21,     // 7 * 3
		Width:   18,     // 6 * 3
		Height:  39,     // 13 * 3
		Ascent:  33,     // 11 * 3
		Descent: 6,      // 2 * 3
		Mask:    basicfont.Face7x13.Mask,
		Ranges: []basicfont.Range{
			{'\u0020', '\u007f', 0},
			{'\ufffd', '\ufffe', 95},
		},
	}

	// Draw the big timer centered
	bounds := font.MeasureString(bigFontFace, displayTime)
	x := (windowWidth - bounds.Round()) / 2
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
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	milliseconds := int(d.Milliseconds()) % 1000

	if minutes > 0 {
		return fmt.Sprintf("%d:%02d.%03d", minutes, seconds, milliseconds)
	}
	return fmt.Sprintf("%d.%03d", seconds, milliseconds)
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