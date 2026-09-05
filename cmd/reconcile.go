package cmd

import (
	"github.com/spf13/cobra"
)

var repo string

// reconcileCmd represents the reconcile command
var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Trigger an immediate reconciliation cycle for a specific repository",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("Triggering reconciliation for repository: %s\n", repo)
		// Integration with argo-c reconcile logic would go here.
	},
}

func init() {
	rootCmd.AddCommand(reconcileCmd)

	reconcileCmd.Flags().StringVarP(&repo, "repo", "r", "", "The repository to reconcile")
	reconcileCmd.MarkFlagRequired("repo")
}
