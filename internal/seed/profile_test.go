package seed

import "testing"

func TestParseConfigRequiresLocalEnvironmentAndKnownProfile(t *testing.T) {

	cases := []struct {
		name    string
		lookup  map[string]string
		wantErr bool
		profile Profile
	}{
		{
			name: "full local",
			lookup: map[string]string{
				"APP_ENV":      "development",
				"SEED_PROFILE": "full-local",
			},
			profile: ProfileFullLocal,
		},
		{
			name: "test is allowed",
			lookup: map[string]string{
				"APP_ENV":      "test",
				"SEED_PROFILE": "foundation",
			},
			profile: ProfileFoundation,
		},
		{
			name: "production rejected",
			lookup: map[string]string{
				"APP_ENV":      "production",
				"SEED_PROFILE": "foundation",
			},
			wantErr: true,
		},
		{
			name: "unknown profile rejected",
			lookup: map[string]string{
				"APP_ENV":      "development",
				"SEED_PROFILE": "saas",
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfig(func(key string) (string, bool) {
				value, ok := tc.lookup[key]
				return value, ok
			})
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseConfig() error = %v", err)
			}
			if cfg.Profile != tc.profile {
				t.Fatalf("profile = %q, want %q", cfg.Profile, tc.profile)
			}
		})
	}
}

func TestProfileIsCumulative(t *testing.T) {
	if !ProfileFoundation.Includes(ProfileFoundation) {
		t.Fatal("foundation must include itself")
	}
	if !ProfilePermissionMatrix.Includes(ProfileFoundation) {
		t.Fatal("permission-matrix must include foundation")
	}
	if !ProfileDemoCMS.Includes(ProfilePermissionMatrix) {
		t.Fatal("demo-cms must include permission-matrix")
	}
	if !ProfileFullLocal.Includes(ProfileDemoCMS) {
		t.Fatal("full-local must include demo-cms")
	}
}
