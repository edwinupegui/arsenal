package cli

import "github.com/spf13/cobra"

func newTodoShowCmd() *cobra.Command  { return &cobra.Command{Use: "show"} }
func newTodoDoneCmd() *cobra.Command   { return &cobra.Command{Use: "done"} }
func newTodoOpenCmd() *cobra.Command   { return &cobra.Command{Use: "open"} }
func newTodoRmCmd() *cobra.Command     { return &cobra.Command{Use: "rm"} }
func newTodoRestoreCmd() *cobra.Command { return &cobra.Command{Use: "restore"} }
func newTodoEditCmd() *cobra.Command   { return &cobra.Command{Use: "edit"} }
func newTodoPurgeCmd() *cobra.Command  { return &cobra.Command{Use: "purge"} }
