package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/insmtx/corekg/internal/bootstrap"
)

func newServerCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the corekg HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			srv := bootstrap.NewServer(addr)

			errCh := make(chan error, 1)
			go func() {
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					errCh <- err
				}
			}()

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			select {
			case err := <-errCh:
				return err
			case <-ctx.Done():
			}

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			return srv.Shutdown(shutdownCtx)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":8080", "HTTP listen address")

	return cmd
}
