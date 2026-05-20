//go:build windows

package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/duckfullstop/blinkybeacon/pkg/fsbeacon"
)

func main() {
	port := flag.Int("port", 1337, "HTTP server port")
	flag.Parse()

	appState := NewAppState()
	httpServer := NewHTTPServer(appState, *port)

	// Start HTTP server in background.
	go func() {
		log.Printf("HTTP server listening on 127.0.0.1:%d", *port)
		if err := httpServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server stopped unexpectedly: %v", err)
		}
	}()

	// USB beacon connection loop — retries every 10s when not found.
	go func() {
		for {
			beacon, err := fsbeacon.OpenFarmBeacon()
			if err != nil {
				log.Printf("Beacon not found, retrying in 10s: %v", err)
				time.Sleep(10 * time.Second)
				continue
			}
			log.Println("Beacon connected")
			appState.SetBeacon(beacon)

			// Wait until something marks the beacon disconnected.
			for {
				_, connected, _ := appState.Get()
				if !connected {
					break
				}
				time.Sleep(2 * time.Second)
			}
			beacon.Close()
		}
	}()

	// Tray runs on the main goroutine and blocks until Quit.
	quit := make(chan struct{})
	RunTray(appState, TrayCallbacks{
		OnSpin: func() {
			_, connected, beacon := appState.Get()
			if !connected {
				return
			}
			if err := beacon.Spin(); err != nil {
				log.Printf("Spin error: %v", err)
				appState.SetBeacon(nil)
				return
			}
			appState.SetState(StateSpin)
		},
		OnFlash: func() {
			_, connected, beacon := appState.Get()
			if !connected {
				return
			}
			if err := beacon.Flash(); err != nil {
				log.Printf("Flash error: %v", err)
				appState.SetBeacon(nil)
				return
			}
			appState.SetState(StateFlash)
		},
		OnStop: func() {
			_, connected, beacon := appState.Get()
			if !connected {
				return
			}
			if err := beacon.Stop(); err != nil {
				log.Printf("Stop error: %v", err)
				appState.SetBeacon(nil)
				return
			}
			appState.SetState(StateIdle)
		},
		OnQuit: func() {
			// Capture and clear the beacon atomically so the USB retry loop exits cleanly.
			_, connected, beacon := appState.Get()
			appState.SetBeacon(nil)
			if connected {
				beacon.Stop()
				beacon.Close()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			httpServer.Shutdown(ctx)
			close(quit)
		},
	})
	<-quit
}
