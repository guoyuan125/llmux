package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudflare/tableflip"
	"github.com/liuguoyuan/llmux/internal/config"
	"github.com/liuguoyuan/llmux/internal/server"
	"github.com/liuguoyuan/llmux/internal/store"
	"github.com/liuguoyuan/llmux/internal/task"
	"github.com/spf13/cobra"
)

var configPath string

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the llmux gateway server",
	Run:   runStart,
}

func init() {
	startCmd.Flags().StringVarP(&configPath, "config", "c", "data/config.yaml", "config file path")
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := store.Init(cfg.Database)
	if err != nil {
		log.Fatalf("failed to init database: %v", err)
	}

	// Graceful restart via tableflip: on SIGUSR2, the new process inherits the
	// listening socket. Existing connections (including active SSE streams) continue
	// on the old process until they complete naturally. This allows rebuilding and
	// restarting llmux without dropping active AI sessions.
	upg, err := tableflip.New(tableflip.Options{})
	if err != nil {
		log.Fatalf("failed to init tableflip: %v", err)
	}
	defer upg.Stop()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGUSR2)
		for range sig {
			log.Println("[upgrade] SIGUSR2 received, starting new process...")
			if err := upg.Upgrade(); err != nil {
				log.Printf("[upgrade] upgrade failed: %v", err)
			}
		}
	}()

	ln, err := upg.Listen("tcp", cfg.Server.Address())
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	runner := task.New(db, cfg)
	runner.Start()
	defer runner.Stop()

	srv := server.New(cfg, db)
	httpServer := &http.Server{
		Handler: srv.Engine(),
	}

	go func() {
		log.Printf("llmux started on %s (pid %d)", cfg.Server.Address(), os.Getpid())
		if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Signal readiness to tableflip: new process is serving, old process can drain.
	if err := upg.Ready(); err != nil {
		log.Fatalf("ready failed: %v", err)
	}

	// Write PID file for `make reload`
	pidFile := "data/llmux.pid"
	os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	defer os.Remove(pidFile)

	<-upg.Exit()

	// Graceful shutdown: wait for active connections to drain (up to 60s for SSE streams)
	log.Printf("[shutdown] draining connections (pid %d)...", os.Getpid())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("[shutdown] server shutdown error: %v", err)
	}
	store.Close(db)
	log.Printf("[shutdown] llmux stopped (pid %d)", os.Getpid())
}
