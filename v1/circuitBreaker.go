package circuitbreaker

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

func (cb *CircuitBreaker) DoRequest(req *http.Request) *CircuitBreakerResponse {
	cb.mutex.Lock()
	state := cb.getState()
	cb.mutex.Unlock()

	if state == Open && cb.fallbackFunc == nil {
		return &CircuitBreakerResponse{
			ResponseType: Error,
			Error: &ErrorDetail{
				Code:    http.StatusServiceUnavailable,
				Message: "Circuit is open",
			},
		}
	} else if state == Open && cb.fallbackFunc != nil {
		return cb.fallbackFunc(req)
	}

	var lastErr error
	for _, interval := range cb.config.RetryIntervals {
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode < 500 {
			cb.recordSuccess()

			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return &CircuitBreakerResponse{
					ResponseType: Error,
					Error: &ErrorDetail{
						Code:    http.StatusInternalServerError,
						Message: "Failed to read response body",
						Raw:     err,
					},
				}
			}

			var data interface{}
			if err := json.Unmarshal(body, &data); err != nil {
				return &CircuitBreakerResponse{
					ResponseType: Error,
					Error: &ErrorDetail{
						Code:    http.StatusInternalServerError,
						Message: "Failed to unmarshal response body",
						Raw:     err,
					},
				}
			}

			return &CircuitBreakerResponse{
				HttpStatus:   resp.StatusCode,
				ResponseType: Success,
				Data:         data,
				Raw:          string(body),
			}
		}
		lastErr = err
		time.Sleep(interval)
		cb.recordFailure()
	}

	if lastErr != nil && cb.fallbackFunc != nil {
		return cb.fallbackFunc(req)
	}

	return &CircuitBreakerResponse{
		ResponseType: Error,
		Error: &ErrorDetail{
			Code:    http.StatusInternalServerError,
			Message: "Failed to execute request",
			Raw:     lastErr,
		},
	}
}

func (cb *CircuitBreaker) SetFallbackFunc(f func(*http.Request) *CircuitBreakerResponse) {
	cb.fallbackFunc = f
}

func (cb *CircuitBreaker) syncStateWithRedis() {
	stateVal, err := cb.redisClient.Get(ctx, cb.name).Result()
	if err == redis.Nil {
		cb.setState(Closed)
	} else if err == nil {
		cb.setState(State(stateVal))
	} else {
		log.Printf("Error fetching state for %s from Redis: %v", cb.name, err)
	}
}

func (cb *CircuitBreaker) setState(state State) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	strState := string(state)
	err := cb.redisClient.Set(ctx, cb.name, strState, 0).Err()
	if err != nil {
		log.Printf("Error updating state for %s in Redis: %v", cb.name, err)
		return
	}
}

func (cb *CircuitBreaker) getState() State {
	stateVal, err := cb.redisClient.Get(ctx, cb.name).Result()
	if err != nil {
		log.Printf("Error fetching state for %s from Redis: %v", cb.name, err)
		return Closed
	}
	return State(stateVal)
}

func (cb *CircuitBreaker) startTimer() {
	state := cb.getState()
	if state == Open {
		cb.timer = time.AfterFunc(cb.config.OpenToHalfOpenWait, func() {
			cb.setState(HalfOpen)
			cb.failures = 0
			cb.success = 0
			log.Println("Circuit breaker transitioned to HALF-OPEN")
		})
	}
}

func (cb *CircuitBreaker) stopTimer() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if cb.timer != nil {
		cb.timer.Stop()
		cb.timer = nil
	}
}

func (cb *CircuitBreaker) recordFailure() {
	state := cb.getState()
	now := time.Now()
	if state == Closed {
		if cb.failures == 0 || now.Sub(cb.lastFail) <= time.Minute {
			cb.failures++
		} else {
			cb.failures = 1
		}
		cb.lastFail = now

		if cb.failures >= cb.config.MaxFailures {
			cb.failures = 0
			cb.setState(Open)
			cb.startTimer()
			log.Println("Circuit breaker transitioned to OPEN")
		}
	} else if state == HalfOpen {
		cb.failures++
		if cb.failures >= cb.config.HalfOpenMaxFailures {
			cb.failures = 0
			cb.success = 0
			cb.setState(Open)
			cb.startTimer()
			log.Println("Circuit breaker transitioned to OPEN from HALF-OPEN due to failures")
		}
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	state := cb.getState()
	if state == HalfOpen {
		cb.success++
		if cb.success >= cb.config.HalfOpenMaxSuccess {
			cb.failures = 0
			cb.success = 0
			cb.setState(Closed)
			cb.stopTimer()
			log.Println("Circuit breaker transitioned to CLOSE from HALF-OPEN due to successes")
		}
	}
}
