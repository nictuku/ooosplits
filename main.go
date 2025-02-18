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
	lastEvent string
	eventTime time.Time
	config    Config
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
		if i < len(g.config.PersonalBest.Splits) {
			splitTime, _ := time.ParseDuration(g.config.PersonalBest.Splits[i].Time)
			totalTime += splitTime
			
			// Format individual split time
			splitTimeStr := formatDuration(splitTime)
			// Format total time
			totalTimeStr := formatDuration(totalTime)
			
			splitLine := fmt.Sprintf("%-25s %10s %10s", splitName, splitTimeStr, totalTimeStr)
			text.Draw(screen, splitLine, fontFace, 50, yPos, white)
			yPos += 20
		}
	}

	text.Draw(screen, "0.00", fontFace, 270, 300, green)

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
			g.lastEvent = "Split"
			g.eventTime = time.Now()
			log.Println("Split triggered")
		case <-hkUndo.Keydown():
			g.lastEvent = "Undo"
			g.eventTime = time.Now()
			log.Println("Undo triggered")
		case <-hkReset.Keydown():
			g.lastEvent = "Reset"
			g.eventTime = time.Now()
			log.Println("Reset triggered")
		}
	}
}