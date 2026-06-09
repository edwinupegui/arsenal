package cli

import (
	"errors"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/config"
	"github.com/edwinupegui/arsenal/internal/configstore"
)

// newConfigCmd returns the `arsenal config` parent command. Subcommands
// (get, set, list, unset) are wired as children. The parent has no RunE —
// invoking it with no subcommand prints the help.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and edit arsenal configuration values",
		Long: "Read and write keys in the arsenal_config table. Keys are " +
			"defined in the config catalog and validated on write.",
	}
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigListCmd())
	cmd.AddCommand(newConfigUnsetCmd())
	return cmd
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Print the value of a config key (or its default if unset)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()
			s := configstore.New(app.DB)
			k := config.Key(args[0])
			v, err := s.GetDefault(cmd.Context(), k)
			if err != nil {
				if errors.Is(err, configstore.ErrUnknownKey) {
					return fmt.Errorf("unknown key %q. Run `arsenal config list` to see valid keys", args[0])
				}
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), v)
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Persist a config value, validating against the catalog",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()
			s := configstore.New(app.DB)
			k := config.Key(args[0])
			if err := s.Set(cmd.Context(), k, args[1]); err != nil {
				if errors.Is(err, configstore.ErrUnknownKey) {
					return fmt.Errorf("unknown key %q. Run `arsenal config list` to see valid keys", args[0])
				}
				if errors.Is(err, configstore.ErrValidation) {
					return fmt.Errorf("invalid value for %q: %s", args[0], err)
				}
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", args[0], args[1])
			return nil
		},
	}
}

func newConfigUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a config key, falling back to the catalog default on next read",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()
			s := configstore.New(app.DB)
			k := config.Key(args[0])
			if err := s.Unset(cmd.Context(), k); err != nil {
				if errors.Is(err, configstore.ErrUnknownKey) {
					return fmt.Errorf("unknown key %q. Run `arsenal config list` to see valid keys", args[0])
				}
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "unset %s\n", args[0])
			return nil
		},
	}
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Print every catalog key with its type, default, current value, and description",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()
			s := configstore.New(app.DB)

			// Fetch current values once; missing keys fall back to the
			// catalog default in the print step.
			current, err := s.All(cmd.Context())
			if err != nil {
				return err
			}

			return printConfigList(cmd.OutOrStdout(), current)
		},
	}
}

// printConfigList renders the config table to w. Split out from the cobra
// command so it is testable without a DB.
func printConfigList(w io.Writer, current map[config.Key]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KEY\tTYPE\tDEFAULT\tCURRENT\tDESCRIPTION")
	for _, k := range config.All() {
		meta, _ := config.GetMeta(k)
		curr, ok := current[k]
		if !ok {
			curr = meta.Default
		}
		desc := meta.Description
		if meta.Type == config.TypeEnum && len(meta.EnumValues) > 0 {
			desc = fmt.Sprintf("%s (allowed: %s)", desc, joinShort(meta.EnumValues))
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", k, meta.Type, meta.Default, curr, desc)
	}
	return tw.Flush()
}

// joinShort is a tiny helper to avoid pulling in strings.Join at the call
// site — keeps the imports section tight.
func joinShort(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
