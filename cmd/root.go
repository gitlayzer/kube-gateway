package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "kube-gateway",
	Short: "A centralized Kubernetes API gateway",
	Long:  "kube-gateway provides a single entry point to manage multiple Kubernetes clusters.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fprintf, err := fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		if err != nil {
			panic(fprintf)
		}
		os.Exit(1)
	}
}
