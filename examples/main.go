package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	circuitbreaker "github.com/shurutech/circuit-breaker/v1"
)

func fallbackFunc(req *http.Request) *circuitbreaker.CircuitBreakerResponse {
	// This is where you define your fallback logic. For example, return a static response or call an alternative service.
	// The following is a simple static response for demonstration purposes.
	return &circuitbreaker.CircuitBreakerResponse{
		HttpStatus:   http.StatusOK,
		ResponseType: circuitbreaker.Fallback,
		Data: map[string]interface{}{
			"message": "This is a fallback response due to circuit breaker open state.",
		},
	}
}

func main() {
	redisOptions := &redis.Options{
		Addr:     "localhost:6379", // Redis server address
		Password: "",               // No password set
		DB:       0,                // Default DB
	}
	rdb := redis.NewClient(redisOptions)

	customConfig := circuitbreaker.Config{
		TimeoutInterval:     5 * time.Second, // Request timeout interval
		MaxFailures:         3,               // Number of failures to open the circuit
		OpenToHalfOpenWait:  1 * time.Minute, // Time to wait before transitioning from OPEN to HALF-OPEN
		HalfOpenMaxSuccess:  2,               // Number of successes to close the circuit from HALF-OPEN
		HalfOpenMaxFailures: 1,               // Number of failures to reopen the circuit from HALF-OPEN
		RetryIntervals: []time.Duration{ // Retry intervals after a failure
			500 * time.Millisecond,
			1 * time.Second,
			2 * time.Second,
		},
	}

	cb := circuitbreaker.NewCircuitBreaker(customConfig, "example", rdb)
	cb.SetFallbackFunc(fallbackFunc)

	requestURL := "http://example.com"
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	response := cb.DoRequest(req)
	if response.Error != nil {
		log.Printf("Request failed with error: %v", response.Error)
		return
	}

	if response.ResponseType == circuitbreaker.Fallback {
		fmt.Println("Fallback response received.")
	} else {
		fmt.Printf("Received response with status code: %d\n", response.HttpStatus)
	}
}
