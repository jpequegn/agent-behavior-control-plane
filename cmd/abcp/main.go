package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jpequegn/agent-behavior-control-plane/internal/audit"
	"github.com/jpequegn/agent-behavior-control-plane/internal/server"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "abcp",
		Short:         "Agent behavior control-plane lab",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(newLedgerCommand(), newServeCommand())
	return root
}

func newLedgerCommand() *cobra.Command {
	ledger := &cobra.Command{
		Use:   "ledger",
		Short: "Read sanitized enforcement decisions from SQLite",
	}
	var databasePath string
	get := &cobra.Command{
		Use:   "get DECISION_ID",
		Short: "Print one sanitized audit decision as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return getLedgerEvent(cmd.Context(), databasePath, args[0], cmd.OutOrStdout())
		},
	}
	get.Flags().StringVar(&databasePath, "db", "abcp-audit.db", "SQLite audit ledger path")
	ledger.AddCommand(get)
	return ledger
}

func getLedgerEvent(ctx context.Context, databasePath, decisionID string, output interface{ Write([]byte) (int, error) }) error {
	ledger, err := audit.Open(databasePath)
	if err != nil {
		return err
	}
	defer ledger.Close()
	event, err := ledger.Get(ctx, decisionID)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(event)
}

func newServeCommand() *cobra.Command {
	var address string
	command := &cobra.Command{
		Use:   "serve",
		Short: "Serve the local control-plane API",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serve(cmd.Context(), address, cmd.OutOrStdout())
		},
	}
	command.Flags().StringVar(&address, "addr", "127.0.0.1:8080", "listen address")
	return command
}

func serve(parent context.Context, address string, output interface{ Write([]byte) (int, error) }) error {
	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: slog.LevelInfo}))
	httpServer := &http.Server{
		Addr:              address,
		Handler:           server.New(logger).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)

	go func() {
		<-stop
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownContext); err != nil {
			logger.Error("shutdown failed", "error", err)
		}
	}()

	logger.Info("control plane listening", "address", address)
	err := httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
