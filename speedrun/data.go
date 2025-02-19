package speedrun

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Split represents a single segment of time in a run
type Split struct {
	Name     string
	Duration time.Duration
}

// Run represents a complete speedrun attempt
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

// RunManager handles all speedrun data operations
type RunManager struct {
	db            *sql.DB
	title         string
	category      string
	attempts      int
	completedRuns int
	splitNames    []string
	splits        []time.Duration
	pb            *Run

	// Run state
	startTime      time.Time
	splitStartTime time.Time
	isRunning      bool
	currentSplit   int
	isCompleted    bool
}

// NewRunManager creates and initializes a new RunManager
func NewRunManager(dbPath string) (*RunManager, error) {
	// Connect to SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Initialize database schema
	if err := initDatabase(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	// Load config from database
	title, category, attempts, completed, splitNames, err := loadConfig(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	// Load personal best
	pb, err := loadPersonalBest(db)
	if err != nil {
		log.Printf("Warning: Failed to load personal best: %v", err)
	}

	return &RunManager{
		db:            db,
		title:         title,
		category:      category,
		attempts:      attempts,
		completedRuns: completed,
		splitNames:    splitNames,
		splits:        make([]time.Duration, 0, len(splitNames)),
		pb:            pb,
	}, nil
}

// Close releases database resources
func (rm *RunManager) Close() error {
	return rm.db.Close()
}

// GetTitle returns the speedrun title
func (rm *RunManager) GetTitle() string {
	return rm.title
}

// GetCategory returns the speedrun category
func (rm *RunManager) GetCategory() string {
	return rm.category
}

// GetAttempts returns the total number of attempts
func (rm *RunManager) GetAttempts() int {
	return rm.attempts
}

// GetCompletedRuns returns the number of completed runs
func (rm *RunManager) GetCompletedRuns() int {
	return rm.completedRuns
}

// GetSplitNames returns the list of split names
func (rm *RunManager) GetSplitNames() []string {
	return rm.splitNames
}

// GetCurrentSplits returns the current split times
func (rm *RunManager) GetCurrentSplits() []time.Duration {
	return rm.splits
}

// GetPersonalBest returns the personal best run
func (rm *RunManager) GetPersonalBest() *Run {
	return rm.pb
}

// IsRunning returns whether a run is in progress
func (rm *RunManager) IsRunning() bool {
	return rm.isRunning
}

// IsCompleted returns whether the current run is completed
func (rm *RunManager) IsCompleted() bool {
	return rm.isCompleted
}

// GetCurrentSplit returns the index of the current split
func (rm *RunManager) GetCurrentSplit() int {
	return rm.currentSplit
}

// GetStartTime returns when the current run started
func (rm *RunManager) GetStartTime() time.Time {
	return rm.startTime
}

// GetSplitStartTime returns when the current split started
func (rm *RunManager) GetSplitStartTime() time.Time {
	return rm.splitStartTime
}

// StartRun begins a new speedrun
func (rm *RunManager) StartRun() {
	rm.isRunning = true
	rm.startTime = time.Now()
	rm.splitStartTime = rm.startTime
	rm.currentSplit = 0
	rm.splits = make([]time.Duration, 0, len(rm.splitNames))
	rm.isCompleted = false
}

// Split records the current split and moves to the next one
// Returns whether this was the final split
func (rm *RunManager) Split() (bool, error) {
	if !rm.isRunning || rm.currentSplit >= len(rm.splitNames) {
		return false, fmt.Errorf("cannot split: run not active or all splits completed")
	}

	// Record split time
	splitDuration := time.Since(rm.splitStartTime)
	rm.splits = append(rm.splits, splitDuration)
	
	isLastSplit := rm.currentSplit == len(rm.splitNames)-1
	if isLastSplit {
		// This was the last split
		rm.isRunning = false
		rm.isCompleted = true
		
		// Save completed run to database
		if err := rm.saveRun(true); err != nil {
			return true, fmt.Errorf("error saving completed run: %v", err)
		}
	} else {
		// Start next split
		rm.currentSplit++
		rm.splitStartTime = time.Now()
	}
	
	return isLastSplit, nil
}

// UndoSplit removes the last split and goes back
func (rm *RunManager) UndoSplit() error {
	if !rm.isRunning || len(rm.splits) == 0 {
		return fmt.Errorf("cannot undo: run not active or no splits recorded")
	}
	
	// Remove last split and go back
	rm.splits = rm.splits[:len(rm.splits)-1]
	rm.currentSplit--
	rm.splitStartTime = time.Now()
	rm.isCompleted = false
	
	return nil
}

// ResetRun cancels the current run
func (rm *RunManager) ResetRun() error {
	if rm.isRunning {
		// Save the unfinished run to database
		if err := rm.saveRun(false); err != nil {
			return fmt.Errorf("error saving unfinished run: %v", err)
		}
	}
	
	// Reset everything
	rm.isRunning = false
	rm.currentSplit = 0
	rm.splits = make([]time.Duration, 0, len(rm.splitNames))
	rm.isCompleted = false
	
	return nil
}

// GetCurrentTime returns the elapsed time of the current run
func (rm *RunManager) GetCurrentTime() time.Duration {
	if !rm.isRunning && len(rm.splits) == 0 {
		return 0
	} else if rm.isCompleted {
		var total time.Duration
		for _, split := range rm.splits {
			total += split
		}
		return total
	} else if rm.isRunning {
		return time.Since(rm.startTime)
	}
	return 0
}

// GetCurrentSplitTime returns the elapsed time of the current split
func (rm *RunManager) GetCurrentSplitTime() time.Duration {
	if !rm.isRunning || rm.currentSplit >= len(rm.splitNames) {
		return 0
	}
	return time.Since(rm.splitStartTime)
}

// Private utility functions

// Helper function to convert Go bool to SQLite int bool
func sqlite3Bool(b bool) int {
	if b {
		return 1
	}
	return 0
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
		SELECT split_name, duration_ns
		FROM splits
		WHERE run_id = ?
		ORDER BY split_index
	`, pb.ID)
	if err != nil {
		return nil, fmt.Errorf("error loading PB splits: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var splitName string
		var durationNs int64
		if err := rows.Scan(&splitName, &durationNs); err != nil {
			return nil, fmt.Errorf("error scanning split data: %v", err)
		}
		pb.Splits = append(pb.Splits, Split{
			Name:     splitName,
			Duration: time.Duration(durationNs),
		})
	}

	return &pb, nil
}

func (rm *RunManager) saveRun(completed bool) error {
	// Calculate end time
	endTime := time.Now()

	// Start transaction
	tx, err := rm.db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Increment attempt counter
	rm.attempts++
	if completed {
		rm.completedRuns++
	}

	// Update config
	_, err = tx.Exec("UPDATE config SET attempts = ?, completed = ? WHERE id = 1", 
		rm.attempts, rm.completedRuns)
	if err != nil {
		return fmt.Errorf("error updating config: %v", err)
	}

	// Insert new run
	result, err := tx.Exec(`
		INSERT INTO runs 
		(title, category, start_time, end_time, completed, is_pb, attempt_num)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		rm.title, rm.category, rm.startTime.Format(time.RFC3339),
		endTime.Format(time.RFC3339), 
		sqlite3Bool(completed), sqlite3Bool(false), rm.attempts,
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
		for _, split := range rm.splits {
			totalTime += split
		}

		// If there's no PB yet or this run is faster, make it the PB
		if rm.pb == nil {
			isPB = true
		} else {
			var pbTotalTime time.Duration
			for _, split := range rm.pb.Splits {
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
	for i, split := range rm.splits {
		_, err = tx.Exec(`
			INSERT INTO splits (run_id, split_index, split_name, duration_ns)
			VALUES (?, ?, ?, ?)
		`, runID, i, rm.splitNames[i], split.Nanoseconds())
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
		rm.pb, err = loadPersonalBest(rm.db)
		if err != nil {
			log.Printf("Warning: Failed to reload PB: %v", err)
		}
	}

	return nil
}

// Update split names
func (rm *RunManager) UpdateSplitNames(names []string) error {
	// Start transaction
	tx, err := rm.db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Delete existing split names
	_, err = tx.Exec("DELETE FROM split_names")
	if err != nil {
		return fmt.Errorf("error deleting existing split names: %v", err)
	}

	// Insert new split names
	for i, name := range names {
		_, err = tx.Exec("INSERT INTO split_names (name, display_order) VALUES (?, ?)", name, i)
		if err != nil {
			return fmt.Errorf("error inserting split name: %v", err)
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	rm.splitNames = names
	return nil
}

// Update run configuration
func (rm *RunManager) UpdateConfig(title, category string) error {
	_, err := rm.db.Exec("UPDATE config SET title = ?, category = ? WHERE id = 1",
		title, category)
	if err != nil {
		return fmt.Errorf("error updating config: %v", err)
	}

	rm.title = title
	rm.category = category
	return nil
}