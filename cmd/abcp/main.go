package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jpequegn/agent-behavior-control-plane/internal/audit"
	"github.com/jpequegn/agent-behavior-control-plane/internal/control"
	"github.com/jpequegn/agent-behavior-control-plane/internal/emergency"
	"github.com/jpequegn/agent-behavior-control-plane/internal/enforce"
	"github.com/jpequegn/agent-behavior-control-plane/internal/flags"
	"github.com/jpequegn/agent-behavior-control-plane/internal/policy"
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
	root.AddCommand(newControlCommand(), newDemoCommand(), newLedgerCommand(), newServeCommand())
	return root
}

func newDemoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "demo incident",
		Short: "Run the synthetic incident safety demo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "incident" {
				return fmt.Errorf("unknown demo %q", args[0])
			}
			return runIncidentDemo(cmd.Context(), cmd.OutOrStdout())
		},
	}
}

func runIncidentDemo(ctx context.Context, output interface{ Write([]byte) (int, error) }) error {
	controls, err := newServerControls()
	if err != nil {
		return err
	}
	flagEvaluator, err := flags.NewEvaluator(controls.Provider(), flags.DefaultCohortConfig())
	if err != nil {
		return err
	}
	ledger, err := audit.Open(":memory:")
	if err != nil {
		return err
	}
	defer ledger.Close()
	engine, err := enforce.NewEngine(flagEvaluator, policy.NewEvaluator(ctx), ledger, control.DefaultIncidentToolCatalog(), enforce.DefaultBudgetLimits())
	if err != nil {
		return err
	}
	engine.WithBoundaryHook(controls)
	restart, err := engine.Execute(ctx, enforce.Request{Environment: control.EnvironmentDevelopment, Proposal: control.IncidentRestartWithoutEvidenceFixture(), RequestedAutonomy: control.AutonomyAct, TrustedInstruction: true})
	if err != nil {
		return err
	}
	_, err = controls.Apply(emergency.Request{Target: flags.FlagGlobalHalt, Value: true, Owner: "demo", Reason: "mid-run halt", ExpiresAt: time.Now().UTC().Add(time.Minute)}, time.Now().UTC())
	if err != nil {
		return err
	}
	halted, err := engine.Execute(ctx, enforce.Request{Environment: control.EnvironmentDevelopment, Proposal: control.IncidentReadFixture(), RequestedAutonomy: control.AutonomyAct, TrustedInstruction: true})
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(map[string]any{"evidenceless_restart": restart.Decision.Decision, "next_boundary_after_halt": halted.Decision.Decision})
}

func newControlCommand() *cobra.Command {
	var endpoint string
	control := &cobra.Command{Use: "control", Short: "Manage emergency controls on a running server"}
	control.PersistentFlags().StringVar(&endpoint, "endpoint", "http://127.0.0.1:8080", "control-plane HTTP endpoint")
	var owner, reason, expiry string
	set := &cobra.Command{
		Use:   "set TARGET VALUE",
		Short: "Set a temporary emergency control",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			expiresAt, err := time.Parse(time.RFC3339, expiry)
			if err != nil {
				return fmt.Errorf("parse expiry: %w", err)
			}
			value := any(args[1])
			if args[1] == "true" {
				value = true
			}
			return mutateControl(cmd.Context(), endpoint, http.MethodPost, "", emergency.Request{Target: args[0], Value: value, Owner: owner, Reason: reason, ExpiresAt: expiresAt}, cmd.OutOrStdout())
		},
	}
	set.Flags().StringVar(&owner, "owner", "", "operator responsible for the control")
	set.Flags().StringVar(&reason, "reason", "", "reason for the control")
	set.Flags().StringVar(&expiry, "expires-at", "", "RFC3339 expiry timestamp")
	list := &cobra.Command{Use: "list", Short: "List active emergency controls", RunE: func(cmd *cobra.Command, _ []string) error {
		return mutateControl(cmd.Context(), endpoint, http.MethodGet, "", nil, cmd.OutOrStdout())
	}}
	clear := &cobra.Command{Use: "clear ID", Short: "Clear an emergency control", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return mutateControl(cmd.Context(), endpoint, http.MethodDelete, args[0], nil, cmd.OutOrStdout())
	}}
	control.AddCommand(set, list, clear)
	return control
}

func mutateControl(ctx context.Context, endpoint, method, id string, value any, output interface{ Write([]byte) (int, error) }) error {
	var body []byte
	if value != nil {
		var err error
		body, err = json.Marshal(value)
		if err != nil {
			return err
		}
	}
	path := strings.TrimRight(endpoint, "/") + "/controls"
	if id != "" {
		path += "/" + id
	}
	request, err := http.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if value != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("control API returned %s", response.Status)
	}
	_, err = io.Copy(output, response.Body)
	return err
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
	controls, err := newServerControls()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: slog.LevelInfo}))
	httpServer := &http.Server{
		Addr:              address,
		Handler:           server.New(logger).WithControls(controls).Handler(),
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
	err = httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func newServerControls() (*emergency.Manager, error) {
	content, err := os.ReadFile("config/flags.json")
	if err != nil {
		return nil, fmt.Errorf("read flag configuration: %w", err)
	}
	config, err := flags.ParseFlagdJSON(content)
	if err != nil {
		return nil, err
	}
	provider, err := flags.NewLocalProvider(config)
	if err != nil {
		return nil, err
	}
	return emergency.NewManager(provider)
}
