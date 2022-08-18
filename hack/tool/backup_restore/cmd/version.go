package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	selfVersion = "v0.1.0"
	tfVersion   = "v0.7.4"
)

// newVersionCmd represents the version command
func newVersionCmd() *cobra.Command {
	versionCmd := &cobra.Command{
		Use: "version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("version: %s\n", selfVersion)
			fmt.Printf("compatible with the latest terraform-controller version: %s\n", tfVersion)
		},
	}
	return versionCmd
}
