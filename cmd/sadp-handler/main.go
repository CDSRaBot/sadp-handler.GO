package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/CDSRaBot/sadp-handler.GO/internal/app"
	"github.com/CDSRaBot/sadp-handler.GO/internal/config"
	"github.com/CDSRaBot/sadp-handler.GO/internal/ui"
)

func main() {
	os.Exit(run())
}

func run() int {
	settings, err := config.Load("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ui.ExitInvalidInput
	}
	baseDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ui.ExitOperational
	}
	runtime, err := app.Compose(settings, baseDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ui.ExitInvalidInput
	}
	logging, err := app.Initialize(settings, filepath.Join(baseDir, "logs"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ui.ExitOperational
	}
	defer logging.Close()

	session := ui.NewM6DiscoverySessionService()
	session.Multicast = runtime.Network
	locales, err := app.NewLocalizationService(runtime.Localization.FS, app.DefaultEnglishCatalog())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ui.ExitOperational
	}
	dependencies := app.BuildDependencies(runtime, app.ServiceSet{
		UI: ui.Dependencies{
			DiscoverySession: session,
			InterfaceLister:  session,
			Locales:          locales,
			Help:             app.DefaultEnglishCatalog()["ui.help"],
			Version:          "dev",
		},
	})
	return ui.RunCLIWithDefaults(
		context.Background(),
		os.Args[1:],
		os.Stdin,
		os.Stdout,
		os.Stderr,
		runtime.UI,
		dependencies.Services.UI,
	)
}
