package speedrun

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"
)

// SpeedrunJSON represents the structure of the JSON input file
type SpeedrunJSON struct {
	Title        string        `json:"title"`
	Category     string        `json:"category"`
	Attempts     int           `json:"attempts"`
	Completed    int           `json:"completed"`
	SplitNames   []string      `json:"split_names"`
	Golds        []interface{} `json:"golds"`
	PersonalBest *PBData       `json:"personal_best"`
}

// PBData represents personal best data in the JSON
type PBData struct {
	Attempt int       `json:"attempt"`
	Splits  []PBSplit `json:"splits"`
}

// PBSplit represents a single split time in the PB
type PBSplit struct {
	Time string `json:"time"`
}

// ImportFromJSON loads speedrun configuration from a JSON file
func (rm *RunManager) ImportFromJSON(filepath string) error {
	// Read JSON file
	jsonData, err := ioutil.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %v", err)
	}

	// Parse JSON
	var speedrun SpeedrunJSON
	if err := json.Unmarshal(jsonData, &speedrun); err != nil {
		return fmt.Errorf("failed to parse JSON: %v", err)
	}

	// Start a transaction
	tx, err := rm.db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Update config
	_, err = tx.Exec("UPDATE config SET title = ?, category = ?, attempts = ?, completed = ? WHERE id = 1",
		speedrun.Title, speedrun.Category, speedrun.Attempts, speedrun.Completed)
	if err != nil {
		return fmt.Errorf("error updating config: %v", err)
	}

	// Delete existing split names
	_, err = tx.Exec("DELETE FROM split_names")
	if err != nil {
		return fmt.Errorf("error deleting existing split names: %v", err)
	}

	// Insert new split names
	for i, name := range speedrun.SplitNames {
		_, err = tx.Exec("INSERT INTO split_names (name, display_order) VALUES (?, ?)", name, i)
		if err != nil {
			return fmt.Errorf("error inserting split name: %v", err)
		}
	}

	// Delete any existing PB
	_, err = tx.Exec("UPDATE runs SET is_pb = 0 WHERE is_pb = 1")
	if err != nil {
		return fmt.Errorf("error resetting previous PB: %v", err)
	}

	// Insert personal best if available
	if speedrun.PersonalBest != nil && len(speedrun.PersonalBest.Splits) > 0 {
		// Use a placeholder start time (24h ago)
		startTime := time.Now().Add(-24 * time.Hour)

		// Calculate split durations and end time
		splits := make([]time.Duration, len(speedrun.PersonalBest.Splits))
		var totalTime time.Duration

		for i, split := range speedrun.PersonalBest.Splits {
			// Parse the time string (expected format: "m:ss.fff" or "ss.fff")
			parts := strings.Split(split.Time, ":")
			var minutes, seconds float64

			if len(parts) == 2 {
				fmt.Sscanf(parts[0], "%f", &minutes)
				fmt.Sscanf(parts[1], "%f", &seconds)
			} else {
				fmt.Sscanf(parts[0], "%f", &seconds)
			}

			// For absolute splits, calculate the individual split duration
			var splitDuration time.Duration
			if i == 0 {
				splitDuration = time.Duration(minutes*60*float64(time.Second) + seconds*float64(time.Second))
			} else {
				currentTotal := time.Duration(minutes*60*float64(time.Second) + seconds*float64(time.Second))
				prevTotal := totalTime
				splitDuration = currentTotal - prevTotal
			}

			splits[i] = splitDuration
			totalTime += splitDuration
		}

		endTime := startTime.Add(totalTime)

		// Insert the PB run
		result, err := tx.Exec(`
			INSERT INTO runs 
			(title, category, start_time, end_time, completed, is_pb, attempt_num)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
			speedrun.Title, speedrun.Category,
			startTime.Format(time.RFC3339), endTime.Format(time.RFC3339),
			sqlite3Bool(true), sqlite3Bool(true), speedrun.PersonalBest.Attempt,
		)
		if err != nil {
			return fmt.Errorf("error inserting PB run: %v", err)
		}

		runID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("error getting last insert ID: %v", err)
		}

		// Insert PB splits
		for i, splitDuration := range splits {
			_, err = tx.Exec(`
				INSERT INTO splits (run_id, split_index, split_name, duration_ns)
				VALUES (?, ?, ?, ?)
			`, runID, i, speedrun.SplitNames[i], splitDuration.Nanoseconds())
			if err != nil {
				return fmt.Errorf("error inserting PB split: %v", err)
			}
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	// Update the RunManager fields
	rm.title = speedrun.Title
	rm.category = speedrun.Category
	rm.attempts = speedrun.Attempts
	rm.completedRuns = speedrun.Completed
	rm.splitNames = speedrun.SplitNames

	// Reload PB
	rm.pb, err = loadPersonalBest(rm.db)
	if err != nil {
		return fmt.Errorf("failed to reload PB after import: %v", err)
	}

	return nil
}
