package rbn

import "time"

const (
	DefaultTelnetBatchSize     = 5
	DefaultTelnetBatchInterval = 5 * time.Second
)

type BatchPolicy struct {
	MaxRecords int
	MaxDelay   time.Duration
}

func DefaultTelnetBatchPolicy() BatchPolicy {
	return BatchPolicy{
		MaxRecords: DefaultTelnetBatchSize,
		MaxDelay:   DefaultTelnetBatchInterval,
	}
}

func (p BatchPolicy) ShouldFlush(records int, firstBufferedAt time.Time, now time.Time) bool {
	if records <= 0 {
		return false
	}
	maxRecords := p.MaxRecords
	if maxRecords <= 0 {
		maxRecords = DefaultTelnetBatchSize
	}
	if records >= maxRecords {
		return true
	}
	maxDelay := p.MaxDelay
	if maxDelay <= 0 {
		maxDelay = DefaultTelnetBatchInterval
	}
	return !firstBufferedAt.IsZero() && !now.Before(firstBufferedAt.Add(maxDelay))
}
