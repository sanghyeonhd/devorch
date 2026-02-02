// Package proxy provides provider proxy and routing functionality
package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Manager manages provider proxies and routing
type Manager struct {
	proxies   map[string]*Proxy
	proxiesMu sync.RWMutex
	client    *http.Client
	logs      []*LogEntry
	logsMu    sync.RWMutex
	config    Config
}

// Config represents proxy configuration
type Config struct {
	MaxConnections  int
	RateLimitPerMin int
	SessionTimeout  time.Duration
	RequireAuth     bool
	LoadBalancing   bool
}

// Proxy represents a provider proxy instance
type Proxy struct {
	ID           string
	Provider     string
	Endpoint     string
	Status       Status
	CreatedTime  time.Time
	LastUsed     time.Time
	RequestCount int64
	SuccessRate  float64
	AvgLatency   time.Duration
}

// Status represents proxy status
type Status string

const (
	StatusActive      Status = "active"
	StatusIdle        Status = "idle"
	StatusError       Status = "error"
	StatusMaintenance Status = "maintenance"
)

// HealthCheck represents proxy health status
type HealthCheck struct {
	ProxyID      string
	Status       Status
	Latency      time.Duration
	LastCheck    time.Time
	ErrorCount   int
	Connectivity bool
}

// RouteRequest represents a routing request
type RouteRequest struct {
	Provider    string
	Model       string
	RequestType string
	Priority    int
	UserID      string
}

// RouteResponse represents routing response
type RouteResponse struct {
	ProxyID   string
	Endpoint  string
	AuthToken string
	RouteTime time.Duration
	Cached    bool
}

// LogEntry represents proxy log entry
type LogEntry struct {
	Timestamp time.Time
	ProxyID   string
	Action    string
	Details   string
	Success   bool
	Latency   time.Duration
}

// NewManager creates a new proxy manager
func NewManager() *Manager {
	return &Manager{
		proxies: make(map[string]*Proxy),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logs: make([]*LogEntry, 0),
		config: Config{
			MaxConnections:  100,
			RateLimitPerMin: 60,
			SessionTimeout:  30 * time.Minute,
			RequireAuth:     true,
			LoadBalancing:   true,
		},
	}
}

// RegisterProxy registers a new provider proxy
func (m *Manager) RegisterProxy(provider string) (*Proxy, error) {
	proxyID := fmt.Sprintf("proxy_%s_%d", provider, time.Now().Unix())
	endpoint := fmt.Sprintf("https://proxy.devorch.com/%s", provider)

	proxy := &Proxy{
		ID:          proxyID,
		Provider:    provider,
		Endpoint:    endpoint,
		Status:      StatusActive,
		CreatedTime: time.Now(),
		LastUsed:    time.Now(),
		SuccessRate: 0.0,
	}

	m.proxiesMu.Lock()
	m.proxies[proxyID] = proxy
	m.proxiesMu.Unlock()

	// Log proxy registration
	m.addLog(proxyID, "register", fmt.Sprintf("Registered %s proxy", provider), true, 5*time.Millisecond)

	return proxy, nil
}

// RouteRequest routes a request to appropriate proxy
func (m *Manager) RouteRequest(req RouteRequest) (*RouteResponse, error) {
	m.proxiesMu.RLock()
	defer m.proxiesMu.RUnlock()

	// Find best proxy for provider
	var bestProxy *Proxy
	var bestScore float64

	for _, proxy := range m.proxies {
		if proxy.Provider == req.Provider && proxy.Status == StatusActive {
			// Calculate routing score (simplified)
			score := proxy.SuccessRate*0.5 +
				(1.0-float64(proxy.AvgLatency.Milliseconds())/1000.0)*0.3 +
				0.2 // base score

			if score > bestScore {
				bestScore = score
				bestProxy = proxy
			}
		}
	}

	if bestProxy == nil {
		return nil, fmt.Errorf("no available proxy for provider %s", req.Provider)
	}

	// Update proxy usage
	bestProxy.LastUsed = time.Now()
	bestProxy.RequestCount++

	response := &RouteResponse{
		ProxyID:   bestProxy.ID,
		Endpoint:  bestProxy.Endpoint,
		AuthToken: "auto-injected-token", // Would be real auth token
		RouteTime: 25 * time.Millisecond, // Simulated routing time
		Cached:    false,
	}

	// Log request routing
	m.addLog(bestProxy.ID, "route_request",
		fmt.Sprintf("Routed %s request to %s", req.RequestType, bestProxy.Provider),
		true, response.RouteTime)

	return response, nil
}

// InjectAuth injects authentication for proxy requests
func (m *Manager) InjectAuth(proxyID, userID string) (string, error) {
	m.proxiesMu.RLock()
	proxy, exists := m.proxies[proxyID]
	m.proxiesMu.RUnlock()

	if !exists {
		return "", fmt.Errorf("proxy %s not found", proxyID)
	}

	// Generate auth token (simplified)
	authToken := fmt.Sprintf("auth_%s_%s_%d", proxy.Provider, userID, time.Now().Unix())

	// Log auth injection
	m.addLog(proxyID, "auth_inject",
		fmt.Sprintf("Injected auth token for user %s", userID),
		true, 10*time.Millisecond)

	return authToken, nil
}

// CheckHealth performs health check on all proxies
func (m *Manager) CheckHealth() map[string]*HealthCheck {
	m.proxiesMu.RLock()
	defer m.proxiesMu.RUnlock()

	healthChecks := make(map[string]*HealthCheck)

	for id, proxy := range m.proxies {
		// Simulate health check
		latency := time.Duration(20+len(proxy.Provider)*5) * time.Millisecond
		isHealthy := proxy.Status == StatusActive

		health := &HealthCheck{
			ProxyID:      id,
			Status:       proxy.Status,
			Latency:      latency,
			LastCheck:    time.Now(),
			ErrorCount:   0,
			Connectivity: isHealthy,
		}

		healthChecks[id] = health

		// Log health check
		status := "success"
		if !isHealthy {
			status = "failed"
		}
		m.addLog(id, "health_check",
			fmt.Sprintf("Health check %s", status),
			isHealthy, latency)
	}

	return healthChecks
}

// GetLogs returns recent proxy logs
func (m *Manager) GetLogs(limit int) []*LogEntry {
	m.logsMu.RLock()
	defer m.logsMu.RUnlock()

	// Return copy of logs
	logs := make([]*LogEntry, len(m.logs))
	copy(logs, m.logs)

	// Sort by timestamp (newest first) - simple bubble sort
	for i := 0; i < len(logs)-1; i++ {
		for j := 0; j < len(logs)-i-1; j++ {
			if logs[j].Timestamp.Before(logs[j+1].Timestamp) {
				logs[j], logs[j+1] = logs[j+1], logs[j]
			}
		}
	}

	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}

	return logs
}

// GetProxies returns all registered proxies
func (m *Manager) GetProxies() map[string]*Proxy {
	m.proxiesMu.RLock()
	defer m.proxiesMu.RUnlock()

	proxies := make(map[string]*Proxy)
	for id, proxy := range m.proxies {
		proxies[id] = proxy
	}

	return proxies
}

// UpdateProxyStatus updates proxy status
func (m *Manager) UpdateProxyStatus(proxyID string, status Status) error {
	m.proxiesMu.Lock()
	defer m.proxiesMu.Unlock()

	proxy, exists := m.proxies[proxyID]
	if !exists {
		return fmt.Errorf("proxy %s not found", proxyID)
	}

	proxy.Status = status
	return nil
}

// RemoveProxy removes a proxy
func (m *Manager) RemoveProxy(proxyID string) error {
	m.proxiesMu.Lock()
	defer m.proxiesMu.Unlock()

	if _, exists := m.proxies[proxyID]; !exists {
		return fmt.Errorf("proxy %s not found", proxyID)
	}

	delete(m.proxies, proxyID)
	return nil
}

// addLog adds a new log entry
func (m *Manager) addLog(proxyID, action, details string, success bool, latency time.Duration) {
	m.logsMu.Lock()
	defer m.logsMu.Unlock()

	logEntry := &LogEntry{
		Timestamp: time.Now(),
		ProxyID:   proxyID,
		Action:    action,
		Details:   details,
		Success:   success,
		Latency:   latency,
	}

	m.logs = append(m.logs, logEntry)

	// Keep only last 1000 log entries
	if len(m.logs) > 1000 {
		m.logs = m.logs[len(m.logs)-1000:]
	}
}

// ClearLogs clears all log entries
func (m *Manager) ClearLogs() {
	m.logsMu.Lock()
	defer m.logsMu.Unlock()
	m.logs = m.logs[:0]
}

// ProxyRequest forwards an HTTP request through the proxy
func (m *Manager) ProxyRequest(req *http.Request, proxyID string) (*http.Response, error) {
	m.proxiesMu.RLock()
	proxy, exists := m.proxies[proxyID]
	m.proxiesMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("proxy %s not found", proxyID)
	}

	startTime := time.Now()

	// Clone request for forwarding
	proxyReq := req.Clone(req.Context())

	// Update URL to proxy endpoint
	proxyURL, err := url.Parse(proxy.Endpoint)
	if err != nil {
		m.addLog(proxyID, "proxy_error",
			fmt.Sprintf("Invalid proxy URL: %s", err.Error()),
			false, time.Since(startTime))
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	proxyReq.URL.Scheme = proxyURL.Scheme
	proxyReq.URL.Host = proxyURL.Host

	// Forward request
	resp, err := m.client.Do(proxyReq)
	latency := time.Since(startTime)

	if err != nil {
		proxy.Status = StatusError
		m.addLog(proxyID, "proxy_request",
			fmt.Sprintf("Request failed: %s", err.Error()),
			false, latency)
		return nil, fmt.Errorf("proxy request failed: %w", err)
	}

	// Update proxy stats
	proxy.LastUsed = time.Now()
	proxy.RequestCount++

	m.addLog(proxyID, "proxy_request",
		fmt.Sprintf("Request successful (%d)", resp.StatusCode),
		true, latency)

	return resp, nil
}

// GetProxyStats returns proxy usage statistics
func (m *Manager) GetProxyStats() map[string]interface{} {
	m.proxiesMu.RLock()
	defer m.proxiesMu.RUnlock()

	totalRequests := int64(0)
	activeProxies := 0

	for _, proxy := range m.proxies {
		totalRequests += proxy.RequestCount
		if proxy.Status == StatusActive {
			activeProxies++
		}
	}

	return map[string]interface{}{
		"total_proxies":  len(m.proxies),
		"active_proxies": activeProxies,
		"total_requests": totalRequests,
		"total_logs":     len(m.logs),
	}
}

// GetConfig returns current proxy configuration
func (m *Manager) GetConfig() Config {
	return m.config
}

// UpdateConfig updates proxy configuration
func (m *Manager) UpdateConfig(config Config) {
	m.config = config
}

// Shutdown gracefully shuts down the proxy manager
func (m *Manager) Shutdown() error {
	// Cleanup connections, save state
	return nil
}
