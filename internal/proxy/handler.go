package proxy

import (
	"context"
	"gproxy/internal/config"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

type contextKey string

const targetURLKey contextKey = "target_url"

type ProxyHandler struct {
	logger     *zap.Logger
	routeTable *config.RouteTable
	proxy      *httputil.ReverseProxy
}

func NewProxyHandler(logger *zap.Logger, rt *config.RouteTable) *ProxyHandler {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   200,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	ph := &ProxyHandler{
		logger:     logger,
		routeTable: rt,
	}

	ph.proxy = &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(r *httputil.ProxyRequest) {
			targetURL, ok := r.In.Context().Value(targetURLKey).(*url.URL)
			if !ok {
				return
			}
			r.SetURL(targetURL)
			r.Out.Host = r.In.Host
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			logger.Error("proxy_forwarding_failed",
				zap.Error(err),
				zap.String("host", r.Host),
				zap.String("path", r.URL.Path),
			)
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		},
	}

	return ph
}

func (ph *ProxyHandler) Handler(w http.ResponseWriter, r *http.Request) {
	host, err := getSubDomain(r.Host)
	if err != nil {
		ph.logger.Debug("invalid_host", zap.Error(err), zap.String("host", r.Host))
		http.Error(w, "Invalid Host", http.StatusBadRequest)
		return
	}

	targetURL, err := ph.routeTable.GetTarget(host)
	if err != nil {
		ph.logger.Debug("route_not_found", zap.String("lookup_key", host))
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	ph.logger.Info("request_received", zap.String("lookup_key", host), zap.String("target", targetURL.String()))

	ctx := context.WithValue(r.Context(), targetURLKey, targetURL)
	ph.proxy.ServeHTTP(w, r.WithContext(ctx))
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
