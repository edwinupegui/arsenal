package web

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Options controls Server behavior.
type Options struct {
	Host    string // bind host; empty → "127.0.0.1"
	Port    int    // bind port; 0 → 7777
	OpenURL bool   // launch the platform browser on Run
	OnReady func(addr string)
	LogTo   io.Writer
}

// Server is a small wrapper around http.Server + chi so the cobra command
// can keep its own concerns separate (flags, signals).
type Server struct {
	srv  *http.Server
	addr string
}

// New constructs a Server. The router wires every UI route plus a
// /static/* handler that serves the embedded CSS and htmx asset.
func New(db *sql.DB, opts Options) *Server {
	if opts.Host == "" {
		opts.Host = "127.0.0.1"
	}
	if opts.Port == 0 {
		opts.Port = 7777
	}

	h := newHandlers(db)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Timeout(15 * time.Second))

	// --- static assets ---
	staticSub, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// --- root redirects to resources ---
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/resources", http.StatusSeeOther)
	})

	// --- resources ---
	r.Get("/resources", h.listResources)
	r.Get("/resources/new", h.newResourceForm)
	r.Post("/resources", h.createResource)

	r.Get("/resources/{id}", h.showResource)
	r.Get("/resources/{id}/edit", h.editResourceForm)
	r.Post("/resources/{id}", h.updateResource)

	r.Post("/resources/{id}/delete", h.softDeleteResource)
	r.Post("/resources/{id}/restore", h.restoreResource)
	r.Post("/resources/{id}/purge", h.purgeResource)
	r.Post("/resources/{id}/star", h.toggleStar)

	// --- search / trash ---
	r.Get("/search", h.searchResources)
	r.Get("/trash", h.trashList)

	// --- todos ---
	h.todoRoutes(r)

	// --- categories / tags ---
	r.Get("/categories", h.listCategories)
	r.Get("/tags", h.listTags)

	addr := net.JoinHostPort(opts.Host, fmt.Sprintf("%d", opts.Port))
	return &Server{
		srv: &http.Server{
			Addr:              addr,
			Handler:           r,
			ReadHeaderTimeout: 5 * time.Second,
		},
		addr: addr,
	}
}

// Addr returns the bind address (host:port) the server is configured for.
func (s *Server) Addr() string { return s.addr }

// Handler returns the underlying http.Handler for testing.
func (s *Server) Handler() http.Handler { return s.srv.Handler }

// Run starts the HTTP server and blocks until ctx is canceled. Auto-opens
// the browser when opts.OpenURL is true.
func (s *Server) Run(ctx context.Context, openURL bool, onReady func(addr string)) error {
	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.srv.Addr, err)
	}

	if onReady != nil {
		onReady(s.srv.Addr)
	}

	if openURL {
		go openInBrowser("http://" + s.srv.Addr)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- s.srv.Serve(ln) }()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.srv.Shutdown(shutCtx)
	}
}

// openInBrowser shells out to the platform-native URL opener. Identical to
// the TUI's helper but kept local so the web package doesn't depend on tui.
func openInBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
