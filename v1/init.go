package circuitbreaker

func NewCircuitBreaker(config Config, name string, rdb RedisClient) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:        name,
		config:      config,
		redisClient: rdb,
	}
	cb.syncStateWithRedis()
	cb.startTimer()
	return cb
}
