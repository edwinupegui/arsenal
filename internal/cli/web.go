package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/web"
)

// newWebCmd builds `arsenal web`. It binds 127.0.0.1:7777 by default —
// localhost-only is intentional. The local DB has no auth; exposing it on a
// LAN address would be a footgun. Power users can override with --host.
func newWebCmd() *cobra.Command {
	var (
		host    string
		port    int
		noOpen  bool
	)
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Run the local web UI on http://127.0.0.1:7777",
		Long: `Web starts a small HTTP server that serves the same data as the CLI and
TUI through an HTMX-enhanced HTML interface. The server binds 127.0.0.1
by default. The browser opens automatically; pass --no-open to skip it.`,
		Example: `  arsenal web
  arsenal web --port 4321
  arsenal web --host 0.0.0.0 --no-open    # only if you really know why`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			srv := web.New(app.DB, web.Options{
				Host: host, Port: port,
			})

			ctx, cancel := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer cancel()

			ready := func(addr string) {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "arsenal web listening on http://%s\n", addr)
				if noOpen {
					fmt.Fprintln(out, "browser auto-open disabled (--no-open)")
				} else {
					fmt.Fprintln(out, "opening browser…")
				}
				fmt.Fprintln(out, "press Ctrl+C to stop")
			}
			return srv.Run(ctx, !noOpen, ready)
		},
	}
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "bind host (default localhost only)")
	cmd.Flags().IntVar(&port, "port", 7777, "bind port")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "do not auto-open the browser")
	return cmd
}
