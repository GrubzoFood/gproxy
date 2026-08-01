package proxy

import (
	"context"
	"gproxy/internal/alb"
	"gproxy/internal/config"
	"net/http"

	"go.uber.org/zap"
)

func Init(logger *zap.Logger, isDev bool) (http.Handler, context.CancelFunc) {
	ctx := context.Background()
	rt := config.NewRouteTable()

	ctxWithCancel, cancelFunc := context.WithCancel(ctx)

	// init aws config
	awsConfig, err := alb.InitAWSConfig(rt, logger, isDev)
	if err != nil {
		logger.Fatal("failed to initialize AWS configuration", zap.Error(err))
	}

	// spawn a goroutine to update the route table asynchronously.
	go awsConfig.Register(ctxWithCancel)

	// create a ProxyHandler
	proxyHandler := NewProxyHandler(logger, rt)

	mux := http.NewServeMux()
	mux.HandleFunc("/", proxyHandler.Handler)

	logger.Info("gproxy initialized")

	return mux, cancelFunc
}
