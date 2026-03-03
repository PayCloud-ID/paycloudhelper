package paycloudhelper

import (
	"context"
	"time"
)

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Component string `json:"component"`
	Status    string `json:"status"` // "healthy", "degraded", "unhealthy"
	Message   string `json:"message,omitempty"`
	Latency   int64  `json:"latency_ms,omitempty"`
}

// HealthCheck represents the overall health check result
type HealthCheck struct {
	AppName   string         `json:"app_name"`
	AppEnv    string         `json:"app_env"`
	Timestamp time.Time      `json:"timestamp"`
	Overall   string         `json:"overall_status"`
	Checks    []HealthStatus `json:"checks"`
}

// CheckHealth performs health checks on all initialized components
// Returns comprehensive health status for Redis, RabbitMQ, and other components
// This is backward compatible - safe to call even if components aren't initialized
func CheckHealth() *HealthCheck {
	hc := &HealthCheck{
		AppName:   GetAppName(),
		AppEnv:    GetAppEnv(),
		Timestamp: time.Now(),
		Overall:   "healthy",
		Checks:    make([]HealthStatus, 0),
	}

	// Check Redis (safe even if not initialized)
	redisHealth := checkRedisHealth()
	hc.Checks = append(hc.Checks, redisHealth)

	// Check RabbitMQ (safe even if not initialized)
	rabbitHealth := checkRabbitMQHealth()
	hc.Checks = append(hc.Checks, rabbitHealth)

	// Check Sentry (safe even if not initialized)
	sentryHealth := checkSentryHealth()
	hc.Checks = append(hc.Checks, sentryHealth)

	// Determine overall status
	for _, check := range hc.Checks {
		if check.Status == "unhealthy" {
			hc.Overall = "unhealthy"
			break
		} else if check.Status == "degraded" && hc.Overall != "unhealthy" {
			hc.Overall = "degraded"
		}
	}

	return hc
}

// checkRedisHealth checks Redis connection and performance
func checkRedisHealth() HealthStatus {
	status := HealthStatus{Component: "redis"}

	if redisPoolClient == nil {
		status.Status = "unhealthy"
		status.Message = "redis client not initialized"
		return status
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := redisPoolClient.Ping(ctx).Result()
	latency := time.Since(start).Milliseconds()

	if err != nil {
		status.Status = "unhealthy"
		status.Message = err.Error()
		LogE("%s redis unhealthy err=%v", buildLogPrefix("checkRedisHealth"), err)
	} else if latency > 1000 {
		status.Status = "degraded"
		status.Message = "high latency detected"
		LogW("%s redis degraded latency_ms=%d", buildLogPrefix("checkRedisHealth"), latency)
	} else {
		status.Status = "healthy"
		LogD("%s redis healthy latency_ms=%d", buildLogPrefix("checkRedisHealth"), latency)
	}

	status.Latency = latency
	return status
}

// checkRabbitMQHealth checks RabbitMQ connection status
func checkRabbitMQHealth() HealthStatus {
	status := HealthStatus{Component: "rabbitmq"}

	if auditTrailMqClient == nil {
		status.Status = "unhealthy"
		status.Message = "rabbitmq client not initialized"
		LogD("%s rabbitmq not initialized", buildLogPrefix("checkRabbitMQHealth"))
		return status
	}

	// Check connection readiness
	auditTrailMqClient.m.Lock()
	isReady := auditTrailMqClient.isReady
	auditTrailMqClient.m.Unlock()

	if !isReady {
		status.Status = "degraded"
		status.Message = "connection not ready"
		LogW("%s rabbitmq degraded connection_not_ready=true", buildLogPrefix("checkRabbitMQHealth"))
		return status
	}

	status.Status = "healthy"
	LogD("%s rabbitmq healthy", buildLogPrefix("checkRabbitMQHealth"))
	return status
}

// checkSentryHealth checks if Sentry client is initialized
func checkSentryHealth() HealthStatus {
	status := HealthStatus{Component: "sentry"}

	if GetSentryClient() == nil {
		status.Status = "unhealthy"
		status.Message = "sentry client not initialized"
		LogD("%s sentry not initialized", buildLogPrefix("checkSentryHealth"))
		return status
	}

	status.Status = "healthy"
	LogD("%s sentry healthy", buildLogPrefix("checkSentryHealth"))
	return status
}

// GetRedisPoolStats returns Redis connection pool statistics
// Safe to call - returns nil if Redis not initialized
func GetRedisPoolStats() *RedisPoolStats {
	if redisPoolClient == nil {
		return nil
	}

	stats := redisPoolClient.PoolStats()

	return &RedisPoolStats{
		TotalConns: int(stats.TotalConns),
		IdleConns:  int(stats.IdleConns),
		StaleConns: int(stats.StaleConns),
		Hits:       stats.Hits,
		Misses:     stats.Misses,
		Timeouts:   stats.Timeouts,
	}
}

// RedisPoolStats represents Redis connection pool statistics
type RedisPoolStats struct {
	TotalConns int    `json:"total_conns"`
	IdleConns  int    `json:"idle_conns"`
	StaleConns int    `json:"stale_conns"`
	Hits       uint32 `json:"hits"`
	Misses     uint32 `json:"misses"`
	Timeouts   uint32 `json:"timeouts"`
}

// GetRedisMetrics is an alias for GetRedisPoolStats for API consistency
// Returns comprehensive Redis connection pool metrics
func GetRedisMetrics() *RedisPoolStats {
	return GetRedisPoolStats()
}

// RedisMetricsDetailed provides extended Redis metrics including calculated ratios
type RedisMetricsDetailed struct {
	PoolStats    *RedisPoolStats `json:"pool_stats"`
	HitRate      float64         `json:"hit_rate_percent"`      // Cache hit rate percentage
	ActiveConns  int             `json:"active_conns"`          // TotalConns - IdleConns
	PoolUtilized float64         `json:"pool_utilized_percent"` // Percentage of pool in use
}

// GetRedisMetricsDetailed returns enhanced metrics with calculated statistics
// Useful for monitoring and capacity planning
func GetRedisMetricsDetailed() *RedisMetricsDetailed {
	stats := GetRedisPoolStats()
	if stats == nil {
		return nil
	}

	detailed := &RedisMetricsDetailed{
		PoolStats:   stats,
		ActiveConns: stats.TotalConns - stats.IdleConns,
	}

	// Calculate hit rate percentage
	totalRequests := stats.Hits + stats.Misses
	if totalRequests > 0 {
		detailed.HitRate = (float64(stats.Hits) / float64(totalRequests)) * 100
	}

	// Calculate pool utilization percentage
	if stats.TotalConns > 0 {
		detailed.PoolUtilized = (float64(detailed.ActiveConns) / float64(stats.TotalConns)) * 100
	}

	return detailed
}
