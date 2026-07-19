package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	root.AddCommand(newServeCommand())
	return root
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
