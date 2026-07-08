package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "drift",
	Short: "Terraform Drift Detector — detect configuration drift without plan/apply",
	Long: `
 ╔══════════════════════════════════════════════════════════════╗
 ║          Terraform Drift Detector                           ║
 ║  Detect infrastructure drift without terraform plan/apply   ║
 ╚══════════════════════════════════════════════════════════════╝

Compare Terraform state files against actual cloud infrastructure
to identify deleted resources, modified attributes, and tag changes.

Supports multiple cloud providers (AWS, Mock) with extensible architecture.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
