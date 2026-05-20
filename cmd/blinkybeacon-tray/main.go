//go:build windows

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"

	"github.com/duckfullstop/blinkybeacon/pkg/fsbeacon"
)

func main() {
	addrFlag := flag.String("addr", "", "IP address to bind the HTTP server (default: saved config or 127.0.0.1)")
	portFlag := flag.Int("port", 0, "HTTP server port (default: saved config or 1337)")
	flag.Parse()

	cfg := loadConfig()
	if *addrFlag != "" {
		cfg.Addr = *addrFlag
	}
	if *portFlag != 0 {
		cfg.Port = *portFlag
	}
	if *addrFlag != "" || *portFlag != 0 {
		if err := saveConfig(cfg); err != nil {
			log.Printf("Warning: could not save config: %v", err)
		}
	}

	appState := NewAppState()

	// restartCh receives a new Config when the user saves settings via /settings.
	restartCh := make(chan Config, 1)

	var httpServer *HTTPServer
	startHTTP := func(c Config) {
		srv := NewHTTPServer(appState, c.Addr, c.Port, func(newCfg Config) {
			select {
			case restartCh <- newCfg:
			default:
			}
		})
		httpServer = srv
		addr := fmt.Sprintf("%s:%d", c.Addr, c.Port)
		appState.SetListenAddr(addr)
		log.Printf("HTTP server listening on %s", addr)
		go func() {
			if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("HTTP server stopped: %v", err)
			}
		}()
	}
	startHTTP(cfg)

	// Watch for config changes from /settings and restart HTTP on the new address.
	go func() {
		for newCfg := range restartCh {
			time.Sleep(300 * time.Millisecond) // let the settings response reach the browser
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			httpServer.Shutdown(ctx)
			cancel()
			startHTTP(newCfg)
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
			beacon.OnError = func() { appState.SetBeacon(nil) }
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
	RunTray(appState, fmt.Sprintf("%s:%d", cfg.Addr, cfg.Port), TrayCallbacks{
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
		OnSettings: func() {
			addr := appState.ListenAddr()
			exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://"+addr+"/settings").Start()
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
			if httpServer != nil {
				httpServer.Shutdown(ctx)
			}
			close(quit)
		},
	})
	<-quit
}
