package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pgpulse",
	Short: "PostgreSQL metrics exporter for Prometheus",
	Long:  "Connects to PostgreSQL, polls pg_stat_activity and pg_stat_statements, and exposes Prometheus metrics on /metrics.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
