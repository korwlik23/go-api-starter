package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestA11(t *testing.T) {
	t.Parallel()

	require.NoError(t, Validate(validSecurityConfig()))

	tests := []struct {
		name          string
		expectedError string
		mutate        func(*Config)
	}{
		{
			name:          "baseline capacity must be positive",
			expectedError: "config: RATE_LIMIT_BASELINE_CAPACITY must be greater than zero",
			mutate: func(cfg *Config) {
				cfg.RateLimits.Baseline.Capacity = 0
			},
		},
		{
			name:          "login window must be positive",
			expectedError: "config: RATE_LIMIT_LOGIN_WINDOW must be greater than zero",
			mutate: func(cfg *Config) {
				cfg.RateLimits.Login.Window = 0
			},
		},
		{
			name:          "recovery capacity must be positive",
			expectedError: "config: RATE_LIMIT_RECOVERY_CAPACITY must be greater than zero",
			mutate: func(cfg *Config) {
				cfg.RateLimits.Recovery.Capacity = -1
			},
		},
		{
			name:          "MFA window must be positive",
			expectedError: "config: RATE_LIMIT_MFA_WINDOW must be greater than zero",
			mutate: func(cfg *Config) {
				cfg.RateLimits.MFA.Window = -time.Second
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := validSecurityConfig()
			test.mutate(&cfg)

			require.EqualError(t, Validate(cfg), test.expectedError)
		})
	}

	strictnessTests := []struct {
		name       string
		policyName string
		mutate     func(*Config)
	}{
		{
			name:       "login equals baseline",
			policyName: "LOGIN",
			mutate: func(cfg *Config) {
				cfg.RateLimits.Login = cfg.RateLimits.Baseline
			},
		},
		{
			name:       "recovery burst exceeds baseline",
			policyName: "RECOVERY",
			mutate: func(cfg *Config) {
				cfg.RateLimits.Recovery = RateLimitConfig{
					Capacity: 121,
					Window:   2 * time.Minute,
				}
			},
		},
		{
			name:       "MFA average rate exceeds baseline",
			policyName: "MFA",
			mutate: func(cfg *Config) {
				cfg.RateLimits.MFA = RateLimitConfig{
					Capacity: 119,
					Window:   30 * time.Second,
				}
			},
		},
	}

	for _, test := range strictnessTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := validSecurityConfig()
			test.mutate(&cfg)

			require.EqualError(
				t,
				Validate(cfg),
				"config: RATE_LIMIT_"+test.policyName+
					" must be stricter than RATE_LIMIT_BASELINE",
			)
		})
	}
}
