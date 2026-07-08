package cmd

import (
	"fmt"
	"time"

	"terraform-drift-detector/internal/models"
	"terraform-drift-detector/internal/scheduler"
	"terraform-drift-detector/internal/store"

	"github.com/spf13/cobra"
)

var (
	schedStateFile string
	schedProvider  string
	schedInterval  string
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Start a scheduled drift scan",
	Long:  "Runs a drift scan on a recurring schedule in the foreground.",
	RunE:  runSchedule,
}

func init() {
	scheduleCmd.Flags().StringVarP(&schedStateFile, "state", "s", "", "Path to Terraform state file (required)")
	scheduleCmd.Flags().StringVarP(&schedProvider, "provider", "p", "mock", "Cloud provider to use (mock, aws)")
	scheduleCmd.Flags().StringVarP(&schedInterval, "interval", "i", "5m", "Scan interval (e.g., 30s, 5m, 1h)")
	scheduleCmd.MarkFlagRequired("state")
	rootCmd.AddCommand(scheduleCmd)
}

func runSchedule(cmd *cobra.Command, args []string) error {
	// Validate interval
	_, err := time.ParseDuration(schedInterval)
	if err != nil {
		return fmt.Errorf("invalid interval %q: %w", schedInterval, err)
	}

	fmt.Printf("\n⚡ Terraform Drift Detector — Scheduler\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  State File : %s\n", schedStateFile)
	fmt.Printf("  Provider   : %s\n", schedProvider)
	fmt.Printf("  Interval   : %s\n", schedInterval)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	s := store.Default()
	sched := scheduler.New(s)

	config := models.ScheduleConfig{
		ID:        fmt.Sprintf("cli-%d", time.Now().Unix()),
		StateFile: schedStateFile,
		Provider:  schedProvider,
		Interval:  schedInterval,
		Enabled:   true,
	}

	if err := sched.AddSchedule(config); err != nil {
		return fmt.Errorf("failed to start schedule: %w", err)
	}

	fmt.Printf("📡 Scheduler running. Press Ctrl+C to stop.\n\n")

	// Block forever
	select {}
}
