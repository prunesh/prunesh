// Module gain: tracks and displays token savings analytics.
// Stores records in ~/.prunesh/gain.db (SQLite).
package gain

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/prunesh/prunesh/internal/storage"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS records (
	id          INTEGER PRIMARY KEY,
	command     TEXT NOT NULL,
	tokens_in   INTEGER NOT NULL,
	tokens_out  INTEGER NOT NULL,
	elapsed_ms  INTEGER NOT NULL,
	recorded_at INTEGER NOT NULL
);
`

// Record stores one command execution.
type Record struct {
	Command    string
	TokensIn   int
	TokensOut  int
	ElapsedMs  int64
	RecordedAt time.Time
}

// Tracker manages the gain database.
type Tracker struct {
	db *sql.DB
}

// Open opens (or creates) the gain database.
func Open() (*Tracker, error) {
	dir, err := storage.Dir()
	if err != nil {
		return nil, fmt.Errorf("data dir: %w", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "gain.db"))
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Tracker{db: db}, nil
}

// Close closes the database.
func (t *Tracker) Close() { t.db.Close() }

// Record saves one command execution.
func (t *Tracker) Record(cmd string, tokensIn, tokensOut int, elapsed time.Duration) error {
	_, err := t.db.Exec(
		`INSERT INTO records (command, tokens_in, tokens_out, elapsed_ms, recorded_at) VALUES (?, ?, ?, ?, ?)`,
		cmd, tokensIn, tokensOut, elapsed.Milliseconds(), time.Now().Unix(),
	)
	return err
}

// Summary aggregates all records.
type Summary struct {
	TotalCommands int
	TokensIn      int
	TokensOut     int
	TokensSaved   int
	SavingsPct    float64
}

// GetSummary returns aggregate stats.
func (t *Tracker) GetSummary() (Summary, error) {
	row := t.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0)
		FROM records WHERE tokens_in > 0
	`)
	var s Summary
	if err := row.Scan(&s.TotalCommands, &s.TokensIn, &s.TokensOut); err != nil {
		return s, err
	}
	s.TokensSaved = s.TokensIn - s.TokensOut
	if s.TokensIn > 0 {
		s.SavingsPct = float64(s.TokensSaved) / float64(s.TokensIn) * 100
	}
	return s, nil
}

// CommandStat is per-command aggregated stats.
type CommandStat struct {
	Command     string
	Count       int
	TokensSaved int
	AvgPct      float64
}

// GetByCommand returns per-command stats sorted by tokens saved.
func (t *Tracker) GetByCommand() ([]CommandStat, error) {
	rows, err := t.db.Query(`
		SELECT command,
		       COUNT(*) as cnt,
		       COALESCE(SUM(tokens_in - tokens_out), 0) as saved,
		       CASE WHEN SUM(tokens_in) > 0
		            THEN CAST(SUM(tokens_in - tokens_out) AS REAL) / SUM(tokens_in) * 100
		            ELSE 0 END as avg_pct
		FROM records
		WHERE tokens_in > 0
		GROUP BY command
		ORDER BY saved DESC
		LIMIT 20
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []CommandStat
	for rows.Next() {
		var s CommandStat
		if err := rows.Scan(&s.Command, &s.Count, &s.TokensSaved, &s.AvgPct); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// PrintSummary prints a human-readable summary to stdout.
func PrintSummary(t *Tracker) error {
	s, err := t.GetSummary()
	if err != nil {
		return err
	}

	bar := progressBar(s.SavingsPct, 20)
	fmt.Printf("prunesh Token Savings\n")
	fmt.Printf("════════════════════════════════════════\n")
	fmt.Printf("Total commands: %d\n", s.TotalCommands)
	fmt.Printf("Tokens in:      %s\n", fmtTokens(s.TokensIn))
	fmt.Printf("Tokens out:     %s\n", fmtTokens(s.TokensOut))
	fmt.Printf("Tokens saved:   %s (%.1f%%)\n", fmtTokens(s.TokensSaved), s.SavingsPct)
	fmt.Printf("Efficiency:     %s %.1f%%\n\n", bar, s.SavingsPct)

	cmds, err := t.GetByCommand()
	if err != nil {
		return err
	}
	if len(cmds) == 0 {
		return nil
	}

	fmt.Printf("By Command\n")
	fmt.Printf("────────────────────────────────────────────────────────\n")
	fmt.Printf("  %-28s %6s %8s %6s\n", "Command", "Count", "Saved", "Avg%")
	fmt.Printf("────────────────────────────────────────────────────────\n")
	for i, c := range cmds {
		cmd := c.Command
		if len(cmd) > 28 {
			cmd = cmd[:25] + "..."
		}
		fmt.Printf("%2d. %-28s %6d %8s %5.1f%%\n",
			i+1, cmd, c.Count, fmtTokens(c.TokensSaved), c.AvgPct)
	}
	fmt.Printf("────────────────────────────────────────────────────────\n")
	return nil
}

func fmtTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func progressBar(pct float64, width int) string {
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	return bar
}
