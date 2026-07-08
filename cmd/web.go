package cmd

import (
	"fmt"
	"log"

	"terraform-drift-detector/internal/store"
	"terraform-drift-detector/web"

	"github.com/spf13/cobra"
)

var webPort int

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Launch the drift detection dashboard",
	Long:  "Starts the web server serving the drift detection API and dashboard UI.",
	RunE:  runWeb,
}

func init() {
	webCmd.Flags().IntVarP(&webPort, "port", "P", 8080, "Port to serve the dashboard on")
	rootCmd.AddCommand(webCmd)
}

func runWeb(cmd *cobra.Command, args []string) error {
	fmt.Printf("\n⚡ Terraform Drift Detector — Dashboard\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  🌐 Starting dashboard at http://localhost:%d\n", webPort)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	s := store.Default()
	addr := fmt.Sprintf(":%d", webPort)

	log.Printf("Dashboard available at http://localhost:%d", webPort)
	return web.StartServer(addr, s)
}
