package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List the cluster list that has been added",
	Run:   runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error getting user home directory: %v", err)
	}
	clustersDir := filepath.Join(home, ".kube-gateway", "clusters")

	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No clusters configured.")
			return
		}
		log.Fatalf("Error reading clusters directory: %v", err)
	}

	fmt.Println("CONFIGURED CLUSTERS:")
	for _, entry := range entries {
		if entry.IsDir() {
			clusterName := entry.Name()
			tokenPath := filepath.Join(clustersDir, clusterName, "token")
			tokenBytes, err := os.ReadFile(tokenPath)
			if err != nil {
				fmt.Printf("  - %s (Error reading token: %v)\n", clusterName, err)
			} else {
				token := strings.TrimSpace(string(tokenBytes))
				fmt.Printf("  - %s (Token: ...%s)\n", clusterName, token[len(token)-8:])
			}
		}
	}
}
