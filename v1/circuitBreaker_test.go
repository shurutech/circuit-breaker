package circuitbreaker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
)

type MockRedisClient struct {
	mock.Mock
}

type RedisClientMock interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	args := m.Called(ctx, key)
	return args.Get(0).(*redis.StringCmd)
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	args := m.Called(ctx, key, value, expiration)
	return args.Get(0).(*redis.StatusCmd)
}

var _ = Describe("CircuitBreaker", func() {
	var (
		cb                *CircuitBreaker
		server            *httptest.Server
		mux               *http.ServeMux
		mockedRedisClient *MockRedisClient
	)

	BeforeEach(func() {
		mux = http.NewServeMux()
		server = httptest.NewServer(mux)

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			data := map[string]interface{}{
				"key": "value",
			}

			w.Header().Set("Content-Type", "application/json")

			err := json.NewEncoder(w).Encode(data)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
		})
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("DoRequest", func() {
		Context("when circuit is CLOSED", func() {
			It("should successfully execute the request", func() {
				mockedRedisClient = new(MockRedisClient)
				var cbState redis.StringCmd
				cbState.SetVal("CLOSED")
				mockedRedisClient.On("Get", mock.Anything, "test").Return(&cbState)
				mockedRedisClient.On("Set", mock.Anything, "test", "CLOSED", mock.Anything).Return(&redis.StatusCmd{})
				cb = NewCircuitBreaker(DefaultConfig, "test", mockedRedisClient)
				req, _ := http.NewRequest("GET", server.URL, nil)
				resp := cb.DoRequest(req)
				Expect(resp.ResponseType).To(Equal(Success))
				Expect(resp.Error).To(BeNil())
			})
		})

		Context("when circuit is OPEN", func() {
			It("should not execute further requests", func() {
				mockedRedisClient = new(MockRedisClient)
				var cbState redis.StringCmd
				cbState.SetVal("OPEN")
				mockedRedisClient.On("Get", mock.Anything, "test").Return(&cbState)
				mockedRedisClient.On("Set", mock.Anything, "test", "OPEN", mock.Anything).Return(&redis.StatusCmd{})
				cb = NewCircuitBreaker(DefaultConfig, "test", mockedRedisClient)
				req, _ := http.NewRequest("GET", server.URL+"/fail", nil)
				resp := cb.DoRequest(req)
				Expect(resp.ResponseType).To(Equal(Error))
				Expect(resp.Error).ToNot(BeNil())
			})
		})

		Context("when request fails and fallback is provided", func() {
			It("should execute the fallback function", func() {
				mockedRedisClient = new(MockRedisClient)
				var cbState redis.StringCmd
				cbState.SetVal("OPEN")
				mockedRedisClient.On("Get", mock.Anything, "test").Return(&cbState)
				mockedRedisClient.On("Set", mock.Anything, "test", "OPEN", mock.Anything).Return(&redis.StatusCmd{})
				cb = NewCircuitBreaker(DefaultConfig, "test", mockedRedisClient)
				cb.SetFallbackFunc(func(req *http.Request) *CircuitBreakerResponse {
					return &CircuitBreakerResponse{
						ResponseType: "fallback",
					}
				})
				req, _ := http.NewRequest("GET", server.URL+"/fail", nil)
				resp := cb.DoRequest(req)
				Expect(resp.ResponseType).To(Equal(Fallback))
			})
		})

		Context("with concurrent requests", func() {
			It("should handle concurrent requests correctly", func() {
				var wg sync.WaitGroup
				var mu sync.Mutex
				successCount := 0
				failureCount := 0

				mockedRedisClient = new(MockRedisClient)
				var cbState redis.StringCmd
				cbState.SetVal("CLOSED")
				mockedRedisClient.On("Get", mock.Anything, "test").Return(&cbState)
				mockedRedisClient.On("Set", mock.Anything, "test", "CLOSED", mock.Anything).Return(&redis.StatusCmd{})
				cb = NewCircuitBreaker(DefaultConfig, "test", mockedRedisClient)
				for i := 0; i < 4; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						req, _ := http.NewRequest("GET", server.URL, nil)
						resp := cb.DoRequest(req)
						mu.Lock()
						defer mu.Unlock()
						if resp.ResponseType == "success" {
							successCount++
						} else {
							failureCount++
						}
					}()
				}
				wg.Wait()
				Expect(successCount + failureCount).To(Equal(4))
			})
		})
	})
})

func TestCircuitBreaker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CircuitBreaker Suite")
}
