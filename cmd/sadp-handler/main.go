package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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
	runtimeDefaults := ui.RuntimeUIConfig{
		Mode:     settings.UI.Mode,
		Format:   settings.UI.Format,
		Language: settings.UI.Language,
		Once:     settings.UI.Once,
		Protocol: settings.Protocol.Strategy,
	}
	runtimeUI, _, err := ui.ParseRuntimeArgsWithDefaults(os.Args[1:], runtimeDefaults)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ui.ExitInvalidInput
	}
	settings.UI.Mode = runtimeUI.Mode
	settings.UI.Format = runtimeUI.Format
	settings.UI.Language = runtimeUI.Language
	settings.UI.Once = runtimeUI.Once
	settings.Protocol.Strategy = runtimeUI.Protocol
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
		Closers: []io.Closer{logging.LogFile},
	})
	defer func() {
		if err := dependencies.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return ui.RunCLIWithDefaults(
		ctx,
		os.Args[1:],
		os.Stdin,
		os.Stdout,
		os.Stderr,
		runtime.UI,
		dependencies.Services.UI,
	)
}
