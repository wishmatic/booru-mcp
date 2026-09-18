package fetch

import "golang.org/x/time/rate"

func NewLimiter(rps float64, burst int) *rate.Limiter {
	if rps <= 0 {
		rps = 1
	}

	if burst <= 0 {
		burst = 1
	}

	return rate.NewLimiter(rate.Limit(rps), burst)
}
