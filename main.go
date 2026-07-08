package main

import (
	"terraform-drift-detector/cmd"
	_ "terraform-drift-detector/internal/provider" // Register all providers
)

func main() {
	cmd.Execute()
}
