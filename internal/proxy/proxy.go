package proxy

import (
	"context"
	"gproxy/internal/alb"
	"gproxy/internal/config"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"go.uber.org/zap"
)

func Init(logger *zap.Logger, isDev bool) (http.Handler, context.CancelFunc) {
	ctx := context.Background()
	rt := config.NewRouteTable()

	ctxWithCancel, cancelFunc := context.WithCancel(ctx)

	awsConfig, err := alb.InitAWSConfig(rt, logger, isDev)
	if err != nil {
		logger.Fatal("failed to initialize AWS configuration", zap.Error(err))
	}

	go awsConfig.Register(ctxWithCancel)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleProxyTraffic(logger, rt, w, r)
	})

	logger.Info("proxy initialized")

	return mux, cancelFunc
}

func handleProxyTraffic(logger *zap.Logger, rt *config.RouteTable, w http.ResponseWriter, r *http.Request) {
	reqLogger := logger.With(
		zap.String("method", r.Method),
		zap.String("host", r.Host),
		zap.String("url", r.URL.String()),
		zap.String("remote_addr", r.RemoteAddr),
	)

	host, err := getSubDomain(r.Host)
	if err != nil {
		reqLogger.Warn("invalid_request", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	targetURL, err := rt.GetTarget(host)
	if err != nil {
		reqLogger.Warn(
			"route_not_found",
			zap.String("lookup_key", host),
		)
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}

	reqLogger.Info(
		"request_received",
		zap.String("lookup_key", host),
		zap.String("target", targetURL.String()),
	)

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Preserve the original host header.
		req.Host = r.Host

		reqLogger.Info(
			"request_forwarded",
			zap.String("destination_host", req.URL.Host),
			zap.String("scheme", req.URL.Scheme),
			zap.String("target_url", req.URL.String()),
		)
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		reqLogger.Info(
			"response_received",
			zap.Int("status_code", resp.StatusCode),
			zap.String("status", resp.Status),
			zap.String("server", resp.Header.Get("Server")),
			zap.String("content_type", resp.Header.Get("Content-Type")),
			zap.Int64("content_length", resp.ContentLength),
		)

		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		reqLogger.Error(
			"proxy_error",
			zap.Error(err),
		)

		http.Error(w, err.Error(), http.StatusBadGateway)
	}

	proxy.ServeHTTP(w, r)
}

func getSubDomain(source string) (string, error) {
	if !strings.Contains(source, "://") {
		source = "https://" + source
	}

	u, err := url.Parse(source)
	if err != nil {
		return "", err
	}

	host := u.Hostname()
	parts := strings.Split(host, ".")

	if len(parts) < 3 {
		return "", nil
	}

	return parts[0], nil
}
