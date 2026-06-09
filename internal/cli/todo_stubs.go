package cli

import "github.com/spf13/cobra"

func newTodoRmCmd() *cobra.Command     { return &cobra.Command{Use: "rm"} }
func newTodoRestoreCmd() *cobra.Command { return &cobra.Command{Use: "restore"} }
func newTodoPurgeCmd() *cobra.Command  { return &cobra.Command{Use: "purge"} }
