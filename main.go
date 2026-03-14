package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/snapcast-client/captive-portal/internal/apmanager"
	"github.com/snapcast-client/captive-portal/internal/connmon"
	"github.com/snapcast-client/captive-portal/internal/portal"
)

const (
	checkInterval   = 15 * time.Second
	apCheckInterval = 30 * time.Second
	defaultPort     = "80"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start web server in all cases (config portal when connected, captive when on AP)
	server, err := portal.NewServer(port)
	if err != nil {
		slog.Error("failed to create portal server", "err", err)
		os.Exit(1)
	}
	server.Start()
	defer server.Stop()

	connMon := connmon.New(logger)
	apMgr := apmanager.New(logger, port)

	// Give NetworkManager time to connect on boot
	time.Sleep(5 * time.Second)

	var inAPMode bool
	tickAP := time.NewTicker(apCheckInterval)
	defer tickAP.Stop()

	for {
		connected, err := connMon.HasConnectivity(ctx)
		if err != nil {
			slog.Warn("connectivity check failed", "err", err)
		}

		if !connected && !inAPMode {
			slog.Info("no connectivity, starting captive portal AP")
			if err := apMgr.StartAP(); err != nil {
				slog.Error("failed to start AP", "err", err)
			} else {
				inAPMode = true
				server.SetCaptiveMode(true)
			}
		} else if connected && inAPMode {
			slog.Info("connectivity restored, stopping AP")
			if err := apMgr.StopAP(); err != nil {
				slog.Warn("failed to stop AP", "err", err)
			}
			inAPMode = false
			server.SetCaptiveMode(false)
		}

		select {
		case <-ctx.Done():
			if inAPMode {
				_ = apMgr.StopAP()
			}
			return
		case <-tickAP.C:
			// next iteration re-checks connectivity
		}
	}
}
