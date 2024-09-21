# Overview

The Circuit Breaker package provides a resilient pattern for making HTTP requests to internal and external services, handling failures gracefully to prevent cascading failures.

# States

The Circuit Breaker package defines three states for managing the flow of requests to internal or external services.

## Closed 🟩

In this default state, requests are sent directly to the service. Failures increment a failure count, which, if it exceeds a configured threshold, triggers a transition to the Open state.

**Note** Failures here refer to the scenario when the request fails due to service unavailability, timeouts, or internal server errors (HTTP status codes 500 and above)

## Open 🛑

The circuit breaker stops all attempts to send requests to the service to prevent failure overload, immediately returning an error for all attempts. After a configurable timeout, it transitions to Half-Open.

## Half-Open ⚠️

Limited numbers of test requests are allowed to pass through. If these requests succeed without an error, the circuit breaker transitions back to Closed, indicating the service is again healthy. If failures occur, it returns to Open.

# Configurations ⚙️

- **TimeoutInterval**: Duration before a request times out.
- **MaxFailures**: Failures threshold for opening the circuit.
- **OpenToHalfOpenWait**: Duration before attempting to reset from Open to Half-Open.
- **HalfOpenMaxSuccess**: Successful requests threshold for closing the circuit from Half-Open.
- **HalfOpenMaxFailures**: Failures threshold for reopening the circuit from Half-Open.
- **RetryIntervals**: Slice of durations for retry intervals between requests.

**Default Configurations**

- **TimeoutInterval:** 10 seconds
- **MaxFailures:** 5
- **OpenToHalfOpenWait:** 30 seconds
- **HalfOpenMaxSuccess:** 5
- **HalfOpenMaxFailures:** 3
- **RetryIntervals: \[**1 seconds, 2 seconds, 3 seconds, 5 seconds, 8 seconds**\]**

# Response Types

The CircuitBreaker package defines three types of response types, each indicating a different outcome of a request processed through the circuit breaker:

## Success

This response type indicates that the request was successfully executed without encountering server errors (HTTP status codes below 500).

E.g

```json
{  
    "http-status": 200,  
    "response-type": "success",  
    "data": {  
        "message": "Data retrieved successfully"  
    }  
}  
```

## Fallback

When the circuit is open (preventing further requests to the external service to avoid overloading), and a fallback function is provided, the response type is marked as Fallback. This type signifies that the response comes from the fallback logic.

E.g

```json
{  
    "http-status": 200,  
    "response-type": "fallback",  
    "data": {  
        "message": "This is a fallback response due to service unavailability."  
    }  
}
```

## Error

This response type is used when the request fails due to service unavailability, timeouts, or internal server errors (HTTP status codes 500 and above). It also covers scenarios where the circuit is open, and no fallback function is provided, indicating that the request cannot be processed.

E.g

```json
{  
    "response-type": "error",  
    "error": {  
        "code": 503,  
        "message": "Circuit is open"  
    }  
}  
```

# Usage

An example is added to the repository to demonstrate the usage of the package. Here is a description of the same

[Code Link](https://github.com/paper-indonesia/pdk/blob/main/example/circuit-breaker/main.go)  
<br/>

```golang
package main

import (
	"fmt"
	"log"
	"net/http"
	"pdk/go/circuitbreaker"
	"time"
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

	cb := circuitbreaker.NewCircuitBreaker(customConfig, "example")
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

```

🎉

#