package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"

	"github.com/ppiankov/pgpulse/internal/config"
	"github.com/ppiankov/pgpulse/internal/snapshot"
)

var (
	statusFormat    string
	statusUnhealthy bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "One-shot health snapshot of the PostgreSQL instance",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusFormat, "format", "json", "Output format (json)")
	statusCmd.Flags().BoolVar(&statusUnhealthy, "unhealthy", false, "Show only problematic items")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	db, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return fmt.Errorf("database open: %w", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}

	snap, err := snapshot.Collect(ctx, db, cfg)
	if err != nil {
		return fmt.Errorf("collect: %w", err)
	}

	if statusUnhealthy {
		snap = snapshot.FilterUnhealthy(snap)
	}

	out, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	fmt.Println(string(out))

	switch snap.Status {
	case snapshot.SeverityDegraded:
		exitCode = 1
	case snapshot.SeverityCritical:
		exitCode = 2
	}

	return nil
}
