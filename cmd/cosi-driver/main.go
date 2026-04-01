package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"k8s.io/klog/v2"

	cosi "sigs.k8s.io/container-object-storage-interface/proto"

	"github.com/espresso-lab/hcloud-cosi-driver/pkg/config"
	"github.com/espresso-lab/hcloud-cosi-driver/pkg/driver"
)

const gracePeriod = 5 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		klog.ErrorS(err, "Invalid configuration")
		os.Exit(1)
	}
	if cfg.DriverName == "" {
		klog.ErrorS(errors.New("empty driver name"), "X_COSI_DRIVER_NAME must not be empty")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		klog.ErrorS(err, "Fatal error")
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config) error {
	identity := &driver.IdentityServer{Name: cfg.DriverName}
	provisioner := &driver.ProvisionerServer{
		AccessKey: cfg.AccessKey,
		SecretKey: cfg.SecretKey,
	}

	srv := grpc.NewServer()
	cosi.RegisterIdentityServer(srv, identity)
	cosi.RegisterProvisionerServer(srv, provisioner)

	lis, cleanup, err := listen(ctx, cfg.COSIEndpoint)
	if err != nil {
		return fmt.Errorf("listener: %w", err)
	}
	defer cleanup()

	var wg sync.WaitGroup
	wg.Add(1)
	go gracefulShutdown(ctx, &wg, srv)

	klog.InfoS("Starting hcloud-cosi-driver", "driver", cfg.DriverName, "endpoint", cfg.COSIEndpoint)
	if err := srv.Serve(lis); err != nil {
		return fmt.Errorf("grpc serve: %w", err)
	}
	wg.Wait()
	return nil
}

func listen(ctx context.Context, endpoint string) (net.Listener, func(), error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("parse endpoint: %w", err)
	}
	if u.Scheme == "unix" {
		_ = os.Remove(u.Path)
	}
	lis, err := (&net.ListenConfig{}).Listen(ctx, u.Scheme, u.Path)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		if u.Scheme == "unix" {
			_ = os.Remove(u.Path)
		}
	}
	return lis, cleanup, nil
}

func gracefulShutdown(ctx context.Context, wg *sync.WaitGroup, srv *grpc.Server) {
	defer wg.Done()
	<-ctx.Done()
	klog.InfoS("Shutting down")

	done := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(gracePeriod):
		klog.InfoS("Grace period exceeded, forcing stop")
		srv.Stop()
	}
}
