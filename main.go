package main

import (
	"database/sql"
	"fmt"
	"image/color"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	_ "github.com/mattn/go-sqlite3"
	"golang.design/x/hotkey"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

const (
	windowWidth   = 600
	windowHeight  = 400
	eventDuration = time.Second
	dbPath        = "speedrun.db"
)

type Split struct {
	Duration time.Duration
}

type Run struct {
	ID         int
	Title      string
	Category   string
	StartTime  time.Time
	EndTime    time.Time
	Completed  bool
	IsPB       bool
	AttemptNum int
	Splits     []Split
}

type Game struct {
	lastEvent      string
	eventTime      time.Time
	title          string
	category       string
	attempts       int
	completedRuns  int
	splitNames     []string
	startTime      time.Time
	splitStartTime time.Time
	isRunning      bool
	currentSplit   int
	splits         []time.Duration
	isCompleted    bool
	db             *sql.DB
	pb             *Run
}

func initDatabase(db *sql.DB) error {
	// Create runs table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			category TEXT NOT NULL,
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP,
			completed BOOLEAN NOT NULL DEFAULT 0,
			is_pb BOOLEAN NOT NULL DEFAULT 0,
			attempt_num INTEGER NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating runs table: %v", err)
	}

	// Create splits table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS splits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL,
			split_index INTEGER NOT NULL,
			split_name TEXT NOT NULL,
			duration_ns INTEGER NOT NULL,
			FOREIGN KEY (run_id) REFERENCES runs(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating splits table: %v", err)
	}

	// Create config table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			title TEXT NOT NULL,
			category TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			completed INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating config table: %v", err)
	}

	// Create split_names table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS split_names (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			display_order INTEGER NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating split_names table: %v", err)
	}

	return nil
}

func loadConfig(db *sql.DB) (string, string, int, int, []string, error) {
	var title, category string
	var attempts, completed int

	// Get config
	row := db.QueryRow("SELECT title, category, attempts, completed FROM config WHERE id = 1")
	err := row.Scan(&title, &category, &attempts, &completed)
	if err != nil {
		if err == sql.ErrNoRows {
			// Default config if not exists
			title = "New Speedrun"
			category = "Any%"
			attempts = 0
			completed = 0
			_, err = db.Exec("INSERT INTO config (id, title, category, attempts, completed) VALUES (1, ?, ?, ?, ?)",
				title, category, attempts, completed)
			if err != nil {
				return "", "", 0, 0, nil, fmt.Errorf("error creating default config: %v", err)
			}
		} else {
			return "", "", 0, 0, nil, fmt.Errorf("error loading config: %v", err)
		}
	}

	// Get split names
	rows, err := db.Query("SELECT name FROM split_names ORDER BY display_order")
	if err != nil {
		return "", "", 0, 0, nil, fmt.Errorf("error loading split names: %v", err)
	}
	defer rows.Close()

	var splitNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return "", "", 0, 0, nil, fmt.Errorf("error scanning split name: %v", err)
		}
		splitNames = append(splitNames, name)
	}

	// If no split names exist, add some defaults
	if len(splitNames) == 0 {
		defaultSplits := []string{"Level 1", "Level 2", "Level 3", "Final Boss"}
		for i, name := range defaultSplits {
			_, err = db.Exec("INSERT INTO split_names (name, display_order) VALUES (?, ?)", name, i)
			if err != nil {
				return "", "", 0, 0, nil, fmt.Errorf("error creating default splits: %v", err)
			}
		}
		splitNames = defaultSplits
	}

	return title, category, attempts, completed, splitNames, nil
}

func loadPersonalBest(db *sql.DB) (*Run, error) {
	// Get the personal best run
	row := db.QueryRow(`
		SELECT id, title, category, start_time, end_time, completed, is_pb, attempt_num
		FROM runs
		WHERE is_pb = 1 AND completed = 1
		LIMIT 1
	`)

	var pb Run
	var startTimeStr, endTimeStr string
	err := row.Scan(
		&pb.ID, &pb.Title, &pb.Category, &startTimeStr, &endTimeStr,
		&pb.Completed, &pb.IsPB, &pb.AttemptNum,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No PB yet
		}
		return nil, fmt.Errorf("error loading personal best: %v", err)
	}

	// Parse timestamps
	pb.StartTime, _ = time.Parse(time.RFC3339, startTimeStr)
	pb.EndTime, _ = time.Parse(time.RFC3339, endTimeStr)

	// Load splits for this PB
	rows, err := db.Query(`
		SELECT duration_ns
		FROM splits
		WHERE run_id = ?
		ORDER BY split_index
	`, pb.ID)
	if err != nil {
		return nil, fmt.Errorf("error loading PB splits: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var durationNs int64
		if err := rows.Scan(&durationNs); err != nil {
			return nil, fmt.Errorf("error scanning split duration: %v", err)
		}
		pb.Splits = append(pb.Splits, Split{Duration: time.Duration(durationNs)})
	}

	return &pb, nil
}

// Helper function to convert Go bool to SQLite int bool
func sqlite3Bool(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (g *Game) saveRun(completed bool) error {
	// Calculate end time
	endTime := time.Now()

	// Start transaction
	tx, err := g.db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Increment attempt counter
	g.attempts++
	if completed {
		g.completedRuns++
	}

	// Update config
	_, err = tx.Exec("UPDATE config SET attempts = ?, completed = ? WHERE id = 1", 
		g.attempts, g.completedRuns)
	if err != nil {
		return fmt.Errorf("error updating config: %v", err)
	}

	// Insert new run
	result, err := tx.Exec(`
		INSERT INTO runs 
		(title, category, start_time, end_time, completed, is_pb, attempt_num)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		g.title, g.category, g.startTime.Format(time.RFC3339),
		endTime.Format(time.RFC3339), 
		sqlite3Bool(completed), sqlite3Bool(false), g.attempts,
	)
	if err != nil {
		return fmt.Errorf("error inserting run: %v", err)
	}

	runID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("error getting last insert ID: %v", err)
	}

	// Check if this is a new personal best
	isPB := false
	if completed {
		var totalTime time.Duration
		for _, split := range g.splits {
			totalTime += split
		}

		// If there's no PB yet or this run is faster, make it the PB
		if g.pb == nil {
			isPB = true
		} else {
			var pbTotalTime time.Duration
			for _, split := range g.pb.Splits {
				pbTotalTime += split.Duration
			}
			isPB = totalTime < pbTotalTime
		}

		if isPB {
			// Reset previous PB flag if exists
			_, err = tx.Exec("UPDATE runs SET is_pb = ? WHERE is_pb = ?", 
				sqlite3Bool(false), sqlite3Bool(true))
			if err != nil {
				return fmt.Errorf("error resetting previous PB: %v", err)
			}

			// Set this run as PB
			_, err = tx.Exec("UPDATE runs SET is_pb = ? WHERE id = ?", 
				sqlite3Bool(true), runID)
			if err != nil {
				return fmt.Errorf("error setting new PB: %v", err)
			}
		}
	}

	// Save splits
	for i, split := range g.splits {
		_, err = tx.Exec(`
			INSERT INTO splits (run_id, split_index, split_name, duration_ns)
			VALUES (?, ?, ?, ?)
		`, runID, i, g.splitNames[i], split.Nanoseconds())
		if err != nil {
			return fmt.Errorf("error inserting split: %v", err)
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	// If this was a PB, reload it
	if isPB {
		g.pb, err = loadPersonalBest(g.db)
		if err != nil {
			log.Printf("Warning: Failed to reload PB: %v", err)
		}
	}

	return nil
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

	// Draw title and category
	text.Draw(screen, g.title, fontFace, 220, 20, white)
	text.Draw(screen, g.category, fontFace, 270, 40, white)

	// Draw attempts
	attemptText := fmt.Sprintf("%d/%d", g.completedRuns, g.attempts)
	text.Draw(screen, attemptText, fontFace, 270, 60, white)

	// Draw splits
	yPos := 100
	var cumulativeTime time.Duration
	var pbCumulativeTime time.Duration

	for i, splitName := range g.splitNames {
		splitTimeStr := "-"
		totalTimeStr := "-"
		diffStr := ""
		diffColor := white
		
		if i < len(g.splits) {
			// This split is completed
			splitTime := g.splits[i]
			cumulativeTime += splitTime
			splitTimeStr = formatDuration(splitTime)
			totalTimeStr = formatDuration(cumulativeTime)
			
			// Compare with PB if available
			if g.pb != nil && i < len(g.pb.Splits) {
				pbSplitTime := g.pb.Splits[i].Duration
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
		} else if i == g.currentSplit && g.isRunning {
			// Current active split
			currentSplitTime := time.Since(g.splitStartTime)
			cumulativeTime += currentSplitTime
			splitTimeStr = formatDuration(currentSplitTime)
			totalTimeStr = formatDuration(cumulativeTime)
			
			// For current split, compare against PB in real-time
			if g.pb != nil && i < len(g.pb.Splits) {
				pbSplitTime := g.pb.Splits[i].Duration
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
		} else if g.pb != nil && i < len(g.pb.Splits) {
			// Show upcoming PB splits
			pbSplitTime := g.pb.Splits[i].Duration
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
	var displayTime string
	if !g.isRunning && len(g.splits) == 0 {
		displayTime = "0:00:00.00"
	} else if g.isCompleted {
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
	// Connect to SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize database schema
	if err := initDatabase(db); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Load config from database
	title, category, attempts, completed, splitNames, err := loadConfig(db)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Load personal best
	pb, err := loadPersonalBest(db)
	if err != nil {
		log.Printf("Warning: Failed to load personal best: %v", err)
	}

	game := &Game{
		title:        title,
		category:     category,
		attempts:     attempts,
		completedRuns: completed,
		splitNames:   splitNames,
		splits:       make([]time.Duration, 0, len(splitNames)),
		db:           db,
		pb:           pb,
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
			} else if g.currentSplit < len(g.splitNames) {
				// Record split time
				splitDuration := time.Since(g.splitStartTime)
				g.splits = append(g.splits, splitDuration)
				
				if g.currentSplit == len(g.splitNames)-1 {
					// This was the last split
					g.isRunning = false
					g.isCompleted = true
					g.lastEvent = "Finished"
					
					// Save completed run to database
					if err := g.saveRun(true); err != nil {
						log.Printf("Error saving completed run: %v", err)
					}
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
				g.isCompleted = false
			}
			g.lastEvent = "Undo"
			g.eventTime = time.Now()
			log.Println("Undo triggered")

		case <-hkReset.Keydown():
			if g.isRunning {
				// Save the unfinished run to database
				if err := g.saveRun(false); err != nil {
					log.Printf("Error saving unfinished run: %v", err)
				}
			}
			
			// Reset everything
			g.isRunning = false
			g.currentSplit = 0
			g.splits = make([]time.Duration, 0, len(g.splitNames))
			g.isCompleted = false
			g.lastEvent = "Reset"
			g.eventTime = time.Now()
			log.Println("Reset triggered")
		}
	}
}