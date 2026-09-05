package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestReconcileCommand(t *testing.T) {
	// Execute the command with a specific repository
	buf := new(bytes.Buffer)

	// Reset the repo flag before each test
	repo = ""

	// Reset the flags so they can be parsed again
	reconcileCmd.ResetFlags()
	reconcileCmd.Flags().StringVarP(&repo, "repo", "r", "", "The repository to reconcile")
	reconcileCmd.MarkFlagRequired("repo")

	// Set args on the root command since we're using that for tests now that it's added
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"reconcile", "--repo", "test-repo"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Unexpected error executing reconcile command: %v", err)
	}

	output := buf.String()
	expectedOutput := "Triggering reconciliation for repository: test-repo"

	if !strings.Contains(output, expectedOutput) {
		t.Errorf("Expected output to contain '%s', got '%s'", expectedOutput, output)
	}
}

func TestReconcileCommandMissingRepo(t *testing.T) {
	buf := new(bytes.Buffer)

	// Reset the repo flag before each test
	repo = ""

	// Reset the flags so they can be parsed again
	reconcileCmd.ResetFlags()
	reconcileCmd.Flags().StringVarP(&repo, "repo", "r", "", "The repository to reconcile")
	reconcileCmd.MarkFlagRequired("repo")

	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"reconcile"}) // No repo flag

	err := rootCmd.Execute()
	if err == nil {
		t.Fatalf("Expected error when repo flag is missing")
	}

	expectedErr := "required flag(s) \"repo\" not set"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedErr, err.Error())
	}
}
