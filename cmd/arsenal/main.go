package main

import (
	"fmt"
	"os"

	"github.com/edwinupegui/arsenal/internal/cli"
	"github.com/edwinupegui/arsenal/internal/migrations"
)

func main() {
	if err := cli.Execute(migrations.FS); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
