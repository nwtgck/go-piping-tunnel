package backoff

import "time"

type exponentialBackoff struct {
	initialDuration time.Duration
	currentDuration time.Duration
	maxDuration     time.Duration
}

func NewExponentialBackoff() *exponentialBackoff {
	initialDuration := 500 * time.Millisecond
	return &exponentialBackoff{
		initialDuration: initialDuration,
		currentDuration: initialDuration,
		maxDuration:     1 * time.Minute,
	}
}

func (b *exponentialBackoff) NextDuration() time.Duration {
	d := b.currentDuration
	nextDuration := time.Duration(float64(b.currentDuration) * 1.5)
	if nextDuration < b.maxDuration {
		b.currentDuration = nextDuration
	}
	return d
}

func (b *exponentialBackoff) Reset() {
	b.currentDuration = b.initialDuration
}
