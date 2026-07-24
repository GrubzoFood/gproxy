package config

import (
	"errors"
	"fmt"
	"net/url"
	"sync"
)

var TargetNotFound error = errors.New("TargetNotFound")

type RouteTable struct {
	mu     sync.RWMutex
	routes map[string]*url.URL
}

func NewRouteTable() *RouteTable {
	return &RouteTable{
		sync.RWMutex{},
		map[string]*url.URL{},
	}
}

func (rt *RouteTable) RegisterRoute(subDomain, target string) error {
	targetURL, err := url.Parse(target)
	if err != nil || subDomain == "" || target == "" {
		return fmt.Errorf("Invalid parameters: %s", err)
	}

	rt.mu.Lock()
	rt.routes[subDomain] = targetURL
	rt.mu.Unlock()
	return nil
}

func (rt *RouteTable) GetTarget(subDomain string) (*url.URL, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if target, exists := rt.routes[subDomain]; exists {
		return target, nil
	}
	return nil, TargetNotFound
}
