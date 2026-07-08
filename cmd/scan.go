package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"terraform-drift-detector/internal/backend"
	"terraform-drift-detector/internal/comparator"
	"terraform-drift-detector/internal/models"
	"terraform-drift-detector/internal/parser"
	_ "terraform-drift-detector/internal/provider" // Register providers
	"terraform-drift-detector/internal/store"

	"github.com/spf13/cobra"
)

var (
	stateFile    string
	providerName string
	jsonOutput   bool
	awsProfile   string
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Run a drift detection scan against a state file",
	Long: `Parses a Terraform state file and compares it against actual cloud infrastructure to detect drift.

Supports local files and S3 remote state:
  drift scan --state terraform.tfstate --provider aws
  drift scan --state s3://my-bucket/env/prod/terraform.tfstate --provider aws
  drift scan --state s3://my-bucket/env/prod/terraform.tfstate --provider aws --profile prod`,
	RunE: runScan,
}

func init() {
	scanCmd.Flags().StringVarP(&stateFile, "state", "s", "", "Path to state file or S3 URI (s3://bucket/key)")
	scanCmd.Flags().StringVarP(&providerName, "provider", "p", "mock", "Cloud provider to use (mock, aws)")
	scanCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output report as JSON")
	scanCmd.Flags().StringVarP(&awsProfile, "profile", "q", "", "AWS credentials profile to use for S3")
	scanCmd.MarkFlagRequired("state")
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	if awsProfile != "" {
		ctx = context.WithValue(ctx, "aws_profile", awsProfile)
	}

	fmt.Printf("\n⚡ Terraform Drift Detector\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  State File : %s\n", stateFile)
	fmt.Printf("  Provider   : %s\n", providerName)
	if awsProfile != "" {
		fmt.Printf("  AWS Profile: %s\n", awsProfile)
	}
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Fetch state file (supports local files and s3:// URIs)
	localPath, err := backend.FetchStateFile(ctx, stateFile, awsProfile)
	if err != nil {
		return fmt.Errorf("failed to fetch state file: %w", err)
	}

	// Parse state file
	resources, err := parser.ParseStateFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	fmt.Printf("📋 Found %d managed resources\n\n", len(resources))

	// Run comparison
	fmt.Printf("🔍 Scanning for drift...\n\n")
	report, err := comparator.Compare(ctx, resources, providerName, stateFile)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Save report
	s := store.Default()
	if err := s.SaveReport(report); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save report: %v\n", err)
	}

	// Output
	if jsonOutput {
		return outputJSON(report)
	}
	return outputPretty(report)
}

func outputJSON(report *models.DriftReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputPretty(report *models.DriftReport) error {
	// Summary
	fmt.Printf("┌─────────────────────────────────────────────┐\n")
	fmt.Printf("│              DRIFT SCAN RESULTS             │\n")
	fmt.Printf("├─────────────────────────────────────────────┤\n")
	fmt.Printf("│  Report ID  : %-28s │\n", report.ID)
	fmt.Printf("│  Timestamp  : %-28s │\n", report.Timestamp)
	fmt.Printf("│  Total      : %-28d │\n", report.TotalResources)
	fmt.Printf("│  ✅ In Sync  : %-27d │\n", report.InSyncCount)
	fmt.Printf("│  ⚠️  Drifted  : %-27d │\n", report.DriftedCount)
	fmt.Printf("│  ❌ Deleted  : %-27d │\n", report.DeletedCount)
	fmt.Printf("└─────────────────────────────────────────────┘\n\n")

	// Per-resource details
	for _, drift := range report.Drifts {
		var statusIcon, statusColor string
		switch drift.Status {
		case models.StatusInSync:
			statusIcon = "✅"
			statusColor = "\033[32m" // Green
		case models.StatusDrifted:
			statusIcon = "⚠️ "
			statusColor = "\033[33m" // Yellow
		case models.StatusDeleted:
			statusIcon = "❌"
			statusColor = "\033[31m" // Red
		}
		reset := "\033[0m"

		fmt.Printf("%s%s %s%s  %s (%s)\n", statusColor, statusIcon, drift.Status, reset, drift.Name, drift.Type)

		if drift.Status == models.StatusDrifted {
			// Show attribute diffs
			for _, ad := range drift.AttributeDiffs {
				fmt.Printf("    📌 %s\n", ad.Name)
				fmt.Printf("       Expected: %s%v%s\n", "\033[32m", ad.Expected, reset)
				fmt.Printf("       Actual:   %s%v%s\n", "\033[31m", ad.Actual, reset)
			}

			// Show tag diffs
			for tagKey, td := range drift.TagDiffs {
				var tagIcon string
				switch td.Status {
				case "modified":
					tagIcon = "🏷️  Modified"
				case "added":
					tagIcon = "🏷️  Added"
				case "removed":
					tagIcon = "🏷️  Removed"
				}
				fmt.Printf("    %s tag %q\n", tagIcon, tagKey)
				if td.Status != "added" {
					fmt.Printf("       Expected: %s%s%s\n", "\033[32m", td.Expected, reset)
				}
				if td.Status != "removed" {
					fmt.Printf("       Actual:   %s%s%s\n", "\033[31m", td.Actual, reset)
				}
			}
			fmt.Println()
		}
	}

	// Footer
	if report.DriftedCount > 0 || report.DeletedCount > 0 {
		fmt.Printf("\n%s⚠️  Drift detected! %d resource(s) need attention.%s\n\n",
			"\033[33m", report.DriftedCount+report.DeletedCount, "\033[0m")
	} else {
		fmt.Printf("\n%s✅ All resources are in sync!%s\n\n",
			"\033[32m", "\033[0m")
	}

	// Tip
	fmt.Printf("💡 Run with --json for machine-readable output\n")
	fmt.Printf("💡 Run 'drift web' to launch the dashboard\n\n")

	_ = strings.Builder{} // prevent unused import

	return nil
}
