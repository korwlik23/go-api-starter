package config

import "math/big"

type namedRateLimit struct {
	name   string
	policy RateLimitConfig
}

func validateRateLimits(cfg Config) error {
	policies := []namedRateLimit{
		{name: "BASELINE", policy: cfg.RateLimits.Baseline},
		{name: "LOGIN", policy: cfg.RateLimits.Login},
		{name: "RECOVERY", policy: cfg.RateLimits.Recovery},
		{name: "MFA", policy: cfg.RateLimits.MFA},
	}

	for _, policy := range policies {
		if policy.policy.Capacity <= 0 {
			return invalidKeyError(
				"RATE_LIMIT_"+policy.name+"_CAPACITY",
				"must be greater than zero",
			)
		}
		if policy.policy.Window <= 0 {
			return invalidKeyError(
				"RATE_LIMIT_"+policy.name+"_WINDOW",
				"must be greater than zero",
			)
		}
	}

	baseline := policies[0].policy
	for _, policy := range policies[1:] {
		if !isStricterRateLimit(policy.policy, baseline) {
			return invalidKeyError(
				"RATE_LIMIT_"+policy.name,
				"must be stricter than RATE_LIMIT_BASELINE",
			)
		}
	}

	return nil
}

func isStricterRateLimit(candidate, baseline RateLimitConfig) bool {
	if candidate.Capacity >= baseline.Capacity {
		return false
	}

	candidateRate := new(big.Int).Mul(
		big.NewInt(int64(candidate.Capacity)),
		big.NewInt(int64(baseline.Window)),
	)
	baselineRate := new(big.Int).Mul(
		big.NewInt(int64(baseline.Capacity)),
		big.NewInt(int64(candidate.Window)),
	)
	return candidateRate.Cmp(baselineRate) < 0
}
