package utils

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsNotExpired(t *testing.T) {
	t.Parallel()
	tests := []struct {
		title           string
		expiresAtIsZero bool
		expiresAtDiff   time.Duration
		margin          time.Duration
		required        bool
		result          bool
	}{
		{
			title:         "future_expiry",
			expiresAtDiff: 5 * time.Minute,
			margin:        10 * time.Second,
			required:      true,
			result:        true,
		},
		{
			title:         "past_expiry",
			expiresAtDiff: -5 * time.Hour,
			margin:        10 * time.Second,
			required:      true,
			result:        false,
		},
		{
			title:         "within_margin",
			expiresAtDiff: 2 * time.Second,
			margin:        10 * time.Second,
			required:      true,
			result:        false,
		},
		{
			title:           "zero_required",
			expiresAtIsZero: true,
			margin:          10 * time.Second,
			required:        true,
			result:          false,
		},
		{
			title:           "zero_not_required",
			expiresAtIsZero: true,
			margin:          10 * time.Second,
			required:        false,
			result:          true,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s_%s", test.title, test.expiresAtDiff.String()), func(t *testing.T) {
			t.Parallel()

			now := time.Now().UTC()
			expiresAt := time.Time{}
			if !test.expiresAtIsZero {
				expiresAt = now.Add(test.expiresAtDiff)
			}
			t.Logf("now: %s, expiresAt: %s\n", now.String(), expiresAt.String())

			result := IsNotExpired(expiresAt, test.margin, test.required)
			assert.Equal(t, test.result, result)
		})
	}
}
