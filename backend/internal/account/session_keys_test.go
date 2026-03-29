package account

import "testing"

func TestRefreshTokenMatchesHash(t *testing.T) {
	t.Parallel()

	rawToken := "refresh-token"
	storedHash := HashRefreshToken(rawToken)

	if !RefreshTokenMatchesHash(rawToken, storedHash) {
		t.Fatalf("expected raw refresh token to match stored hash")
	}

	if RefreshTokenMatchesHash(storedHash, storedHash) {
		t.Fatalf("expected hashed input to not match stored hash")
	}
}
