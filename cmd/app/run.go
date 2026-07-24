package main

import (
	"context"
	"errors"
	"gproxy/internal/proxy"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

const (
	certFile = "./certs/_wildcard.grubzo.food+5.pem"
	keyFile  = "./certs/_wildcard.grubzo.food+5-key.pem"
)

func run(logger *zap.Logger) {
	handler, cancel := proxy.Init(logger, false)

	server := &http.Server{
		Addr:    ":443",
		Handler: handler,
	}

	go func() {
		logger.Info("starting HTTPS proxy",
			zap.String("addr", server.Addr),
			zap.String("cert", certFile),
		)

		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil &&
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
