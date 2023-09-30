package ratelimiter

import (
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/tj/assert"
)

func TestRateLimit(t *testing.T) {
	rl := New(5, time.Millisecond)
	id := ksuid.New().String()
	for i := 0; i < 5; i++ {
		err := rl.Check(id)
		assert.NoError(t, err)
	}

	err := rl.Check(id)
	assert.EqualError(t, err, ErrRateLimitExceeded.Error())

	time.Sleep(time.Millisecond)
	err = rl.Check(id)
	assert.NoError(t, err)
}
