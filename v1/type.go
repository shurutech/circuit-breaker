package circuitbreaker

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	TimeoutInterval     time.Duration
	MaxFailures         int
	OpenToHalfOpenWait  time.Duration
	HalfOpenMaxSuccess  int
	HalfOpenMaxFailures int
	RetryIntervals      []time.Duration
}

var DefaultConfig = Config{
	TimeoutInterval:     10 * time.Second,
	MaxFailures:         5,
	OpenToHalfOpenWait:  30 * time.Second,
	HalfOpenMaxSuccess:  5,
	HalfOpenMaxFailures: 3,
	RetryIntervals:      []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second, 5 * time.Second, 8 * time.Second},
}

type State string

const (
	Closed   State = "CLOSED"
	Open     State = "OPEN"
	HalfOpen State = "HALF OPEN"
)

type ResponseType string

const (
	Success  ResponseType = "success"
	Fallback ResponseType = "fallback"
	Error    ResponseType = "error"
)

type CircuitBreaker struct {
	name         string
	mutex        sync.Mutex
	config       Config
	failures     int
	success      int
	lastFail     time.Time
	timer        *time.Timer
	fallbackFunc func(*http.Request) *CircuitBreakerResponse
	redisClient  RedisClient
}

type ErrorDetail struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Raw     error  `json:"-"`
}

type CircuitBreakerResponse struct {
	HttpStatus   int          `json:"http-status"`
	ResponseType ResponseType `json:"response-type"`
	Data         interface{}  `json:"data,omitempty"`
	Error        *ErrorDetail `json:"error,omitempty"`
	Raw          interface{}  `json:"raw,omitempty"`
}

type RedisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
}
