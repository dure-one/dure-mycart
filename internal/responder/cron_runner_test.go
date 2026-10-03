package responder

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dure-one/dure-mycart/internal/models"
)

func TestCronRunner_shouldRun(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		job      *models.CrontabJob
		expected bool
	}{
		{
			name: "never run before - should run",
			job: &models.CrontabJob{
				Core:     models.Core{ID: "job1"},
				Enabled:  true,
				Interval: "5min",
				LastRun:  nil,
			},
			expected: true,
		},
		{
			name: "disabled - should not run",
			job: &models.CrontabJob{
				Core:     models.Core{ID: "job2"},
				Enabled:  false,
				Interval: "5min",
				LastRun:  nil,
			},
			expected: false,
		},
		{
			name: "last run 6 minutes ago, interval 5min - should run",
			job: &models.CrontabJob{
				Core:     models.Core{ID: "job3"},
				Enabled:  true,
				Interval: "5min",
				LastRun:  ptrInt64(now.Add(-6 * time.Minute).Unix()),
			},
			expected: true,
		},
		{
			name: "last run 3 minutes ago, interval 5min - should not run",
			job: &models.CrontabJob{
				Core:     models.Core{ID: "job4"},
				Enabled:  true,
				Interval: "5min",
				LastRun:  ptrInt64(now.Add(-3 * time.Minute).Unix()),
			},
			expected: false,
		},
		{
			name: "last run 25 hours ago, interval daily - should run",
			job: &models.CrontabJob{
				Core:     models.Core{ID: "job5"},
				Enabled:  true,
				Interval: "daily",
				LastRun:  ptrInt64(now.Add(-25 * time.Hour).Unix()),
			},
			expected: true,
		},
	}

	runner := &CronRunner{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runner.shouldRun(tt.job, now)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCronRunner_parseInterval(t *testing.T) {
	tests := []struct {
		interval string
		expected time.Duration
	}{
		{"5min", 5 * time.Minute},
		{"15min", 15 * time.Minute},
		{"1hr", 1 * time.Hour},
		{"6hr", 6 * time.Hour},
		{"daily", 24 * time.Hour},
		{"invalid", 5 * time.Minute}, // fallback to 5min
	}

	runner := &CronRunner{}
	for _, tt := range tests {
		t.Run(tt.interval, func(t *testing.T) {
			result := runner.parseInterval(tt.interval)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCronRunner_Start(t *testing.T) {
	t.Skip("Integration test - requires database")

	runner := &CronRunner{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := runner.Start(ctx)
	require.NoError(t, err)
}

func ptrInt64(v int64) *int64 {
	return &v
}
