package main

import (
	"context"
	"crypto/tls"
	"embed"
	"errors"
	"gproxy/internal/proxy"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

//go:embed certs/_wildcard.grubzo.food+5.pem certs/_wildcard.grubzo.food+5-key.pem
var certFS embed.FS

func run(logger *zap.Logger) {

	isDev := os.Getenv("GPROXY_DEV") == "true"
	if isDev {
		logger.Info("gproxy is running in dev mode")
	}

	handler, cancel := proxy.Init(logger, isDev)

	certPEM, err := certFS.ReadFile("certs/_wildcard.grubzo.food+5.pem")
	if err != nil {
		logger.Fatal("failed to read embedded cert", zap.Error(err))
	}

	keyPEM, err := certFS.ReadFile("certs/_wildcard.grubzo.food+5-key.pem")
	if err != nil {
		logger.Fatal("failed to read embedded key", zap.Error(err))
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		logger.Fatal("failed to parse embedded cert/key pair", zap.Error(err))
	}

	server := &http.Server{
		Addr:    ":443",
		Handler: handler,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}

	go func() {
		logger.Info("starting HTTPS proxy",
			zap.String("addr", server.Addr),
		)
		// Empty strings: cert/key already loaded via TLSConfig above
		if err := server.ListenAndServeTLS("", ""); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("HTTPS server failed", zap.Error(err))
		}
	}()

	waitSIGINT(logger)
	logger.Info("shutdown initiated")
	cancel()

	ctx, ctxCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer ctxCancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	} else {
		logger.Info("HTTPS proxy stopped gracefully")
	}
}

func waitSIGINT(logger *zap.Logger) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("received signal",
		zap.String("signal", sig.String()),
	)
	signal.Stop(quit)
	close(quit)
}
