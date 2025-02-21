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
	Name        string
	Duration    time.Duration
	// NEW: Holds the best (gold) segment time across *all* runs for this split index.
	// This is computed in memory by scanning the DB. Not necessarily from this run.
	BestSegment time.Duration
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

	rm := &RunManager{
		db:            db,
		title:         title,
		category:      category,
		attempts:      attempts,
		completedRuns: completed,
		splitNames:    splitNames,
		splits:        make([]time.Duration, 0, len(splitNames)),
		pb:            pb,
	}

	// NEW: If we have a PB, also compute the best (gold) segment times
	if pb != nil {
		if err := rm.ComputeBestSegments(); err != nil {
			log.Printf("Warning: Could not compute best segments: %v", err)
		}
	}

	return rm, nil
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

// =====================
// NEW: Best Segments (Gold Splits)
// =====================

// ComputeBestSegments looks at *all* completed runs and finds the minimum segment
// time for each split index. It stores that "gold" time in rm.pb.Splits[i].BestSegment.
// If you want to store gold times in the DB, you'd need a new table or column. Here, we
// do it purely in memory for display.
func (rm *RunManager) ComputeBestSegments() error {
	if rm.pb == nil || len(rm.pb.Splits) == 0 {
		// no PB or no splits
		return nil
	}
	numSplits := len(rm.splitNames)
	bestSegments := make([]time.Duration, numSplits)
	// Initialize them to a large value
	for i := 0; i < numSplits; i++ {
		bestSegments[i] = time.Duration(1<<63 - 1) // big
	}

	// Query all completed runs + their splits
	rows, err := rm.db.Query(`
		SELECT splits.split_index, splits.duration_ns
		FROM splits
		JOIN runs ON splits.run_id = runs.id
		WHERE runs.completed = 1
	`)
	if err != nil {
		return fmt.Errorf("ComputeBestSegments: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var idx int
		var durNs int64
		if err := rows.Scan(&idx, &durNs); err != nil {
			return fmt.Errorf("ComputeBestSegments scan: %v", err)
		}
		d := time.Duration(durNs)
		if idx >= 0 && idx < numSplits && d < bestSegments[idx] {
			bestSegments[idx] = d
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Now fill in the PB run's BestSegment field
	for i := range rm.pb.Splits {
		rm.pb.Splits[i].BestSegment = bestSegments[i]
	}

	return nil
}

// =====================
// NEW: Compare runs to PB
// =====================

// IsBetterThanPB returns true if the *current run* is better (less total time)
// than the stored PB. If there is no PB, it returns true if the current run
// is completed, false otherwise.
func (rm *RunManager) IsBetterThanPB() bool {
	if !rm.isCompleted {
		// not finished
		return false
	}
	var currentTotal time.Duration
	for _, seg := range rm.splits {
		currentTotal += seg
	}
	if rm.pb == nil {
		// no PB in DB, so if we completed, it's automatically "better"
		return true
	}
	var pbTotal time.Duration
	for _, seg := range rm.pb.Splits {
		pbTotal += seg.Duration
	}
	return currentTotal < pbTotal
}

// SaveAsPB forces the last completed run to become PB, even if it's slower.
// Typically you'd only call this if IsBetterThanPB() is true, but you can do
// it unconditionally if you want to override your PB.
func (rm *RunManager) SaveAsPB() error {
	if !rm.isCompleted {
		return fmt.Errorf("cannot save as PB: run not completed")
	}
	// We'll assume the last run we saved is the one we want to set as PB.
	// That means we need to find that run's ID in the DB. If you want to
	// store it in a field, you can. For simplicity, let's just take the
	// largest run_id for which completed=1.
	tx, err := rm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unset old PB
	if _, err = tx.Exec(`UPDATE runs SET is_pb = 0 WHERE is_pb = 1`); err != nil {
		return fmt.Errorf("error resetting old PB: %v", err)
	}

	// Find the latest completed run
	row := tx.QueryRow(`
		SELECT id 
		FROM runs
		WHERE completed = 1
		ORDER BY id DESC
		LIMIT 1
	`)
	var lastCompletedID int64
	if err := row.Scan(&lastCompletedID); err != nil {
		return fmt.Errorf("error finding last completed run: %v", err)
	}

	// Mark it as PB
	if _, err := tx.Exec(`UPDATE runs SET is_pb = 1 WHERE id = ?`, lastCompletedID); err != nil {
		return fmt.Errorf("error setting new PB: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	// Reload PB so rm.pb is up to date
	newPB, err := loadPersonalBest(rm.db)
	if err != nil {
		return fmt.Errorf("error reloading PB: %v", err)
	}
	rm.pb = newPB

	// Also re-compute gold splits if you want them updated
	if err := rm.ComputeBestSegments(); err != nil {
		log.Printf("Warning: Could not re-compute best segments after SaveAsPB: %v", err)
	}

	return nil
}

// =====================
// Private / existing code
// =====================

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

	// Check if this is a new personal best (by total time)
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
			_, err = tx.Exec("UPDATE runs SET is_pb = 0 WHERE is_pb = 1")
			if err != nil {
				return fmt.Errorf("error resetting previous PB: %v", err)
			}

			// Set this run as PB
			_, err = tx.Exec("UPDATE runs SET is_pb = 1 WHERE id = ?", runID)
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
		} else {
			// Recompute gold splits so rm.pb.Splits[i].BestSegment is up to date
			if err := rm.ComputeBestSegments(); err != nil {
				log.Printf("Warning: Could not compute best segments: %v", err)
			}
		}
	}

	return nil
}

// UpdateSplitNames replaces the current split names with a new set
func (rm *RunManager) UpdateSplitNames(names []string) error {
	tx, err := rm.db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM split_names")
	if err != nil {
		return fmt.Errorf("error deleting existing split names: %v", err)
	}

	for i, name := range names {
		_, err = tx.Exec("INSERT INTO split_names (name, display_order) VALUES (?, ?)", name, i)
		if err != nil {
			return fmt.Errorf("error inserting split name: %v", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	rm.splitNames = names
	return nil
}

// UpdateConfig changes the run title/category in the DB and updates memory
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
