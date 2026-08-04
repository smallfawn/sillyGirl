package core

import "testing"

func TestCredentialExpiryDurations(t *testing.T) {
	if adminJWTExpireSeconds != 3*24*60*60 {
		t.Fatalf("admin JWT expiry = %d, want 3 days", adminJWTExpireSeconds)
	}
	if userJWTExpireSeconds != 7*24*60*60 {
		t.Fatalf("user JWT expiry = %d, want 7 days", userJWTExpireSeconds)
	}
}

func TestJWTClaimsExpiredUsesCurrentPolicyMaximum(t *testing.T) {
	const now = int64(2_000_000_000)
	const day = int64(24 * 60 * 60)

	if !jwtClaimsExpired(now, now-8*day, now+22*day, userJWTExpireSeconds) {
		t.Fatal("legacy 30-day user token should expire after the current 7-day maximum")
	}
	if jwtClaimsExpired(now, now-day, now+6*day, userJWTExpireSeconds) {
		t.Fatal("recent user token should remain valid inside the 7-day maximum")
	}
	if !jwtClaimsExpired(now, now-day, now, userJWTExpireSeconds) {
		t.Fatal("token exp claim should still expire at its explicit deadline")
	}
}
