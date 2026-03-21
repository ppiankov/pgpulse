package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ppiankov/pgpulse/internal/doctor"
)

var doctorFormat string

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check runtime readiness for metrics collection",
	RunE:  runDoctor,
}

func init() {
	doctorCmd.Flags().StringVar(&doctorFormat, "format", "json", "Output format (json)")
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		return fmt.Errorf("PG_DSN or DATABASE_URL must be set")
	}

	ctx := context.Background()
	report := doctor.RunAll(ctx, dsn, version)

	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	fmt.Println(string(out))

	switch report.Status {
	case doctor.StatusWarn:
		exitCode = 1
	case doctor.StatusFail:
		exitCode = 2
	}

	return nil
}
