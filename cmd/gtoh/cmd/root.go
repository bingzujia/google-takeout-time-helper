package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gtoh",
	Short: "Google Takeout helper - migrate photo metadata",
	Long:  `gtoh is a cross-platform CLI tool to fix timestamps and organize photos from Google Takeout.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// cobra already prints the error
	}
}

func init() {
	rootCmd.AddCommand(fixTakeoutCmd)
	rootCmd.AddCommand(fixImgCmd)
	rootCmd.AddCommand(organizeCmd)
	rootCmd.AddCommand(renameCmd)
	rootCmd.AddCommand(cleanJSONCmd)
}
