package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "llmux",
	Short: "LLM API aggregation gateway",
	Long:  "llmux is an LLM API aggregation and load balancing gateway for individuals and small teams.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
