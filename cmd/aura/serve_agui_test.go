package main

import (
	"testing"

	"github.com/chetto1983/aura/internal/config"
)

// TestAGUIServerConfigReportsWhetherPoliciesAreEnforced pins the wiring behind the
// cockpit's notice: GET /api/me reports ServerConfig.PoliciesEnforced, and the daemon
// sets it from the deployment profile, false exactly where gateway.Decide skips the
// policies and the standing grants.
func TestAGUIServerConfigReportsWhetherPoliciesAreEnforced(t *testing.T) {
	for _, tc := range []struct {
		profile config.RuntimeProfile
		want    bool
	}{
		{config.ProfileDev, false},
		{config.ProfileLocalTrusted, false},
		{config.ProfileSingleUserHardened, true},
		{config.ProfileServerProduction, true},
	} {
		t.Run(string(tc.profile), func(t *testing.T) {
			got := aguiServerConfig(&config.Config{Profile: tc.profile}, nil, nil, nil).PoliciesEnforced
			if got != tc.want {
				t.Fatalf("PoliciesEnforced = %v under %s, want %v", got, tc.profile, tc.want)
			}
		})
	}
}
