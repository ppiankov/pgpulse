package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "pgpulse",
		Short: "PostgreSQL metrics exporter for Prometheus",
		Long:  "Connects to PostgreSQL, polls pg_stat_activity and pg_stat_statements, and exposes Prometheus metrics on /metrics.",
	}
	exitCode int
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
