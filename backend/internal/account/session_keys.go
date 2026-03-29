package account

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func SessionCacheKey(accountID uint) string {
	return fmt.Sprintf("account:%d", accountID)
}

func RefreshHashKey(accountID uint) string {
	return fmt.Sprintf("account:%d:refresh", accountID)
}

func HashRefreshToken(refreshToken string) string {
	sum := sha256.Sum256([]byte(refreshToken))
	return hex.EncodeToString(sum[:])
}

func RefreshTokenMatchesHash(refreshToken string, storedHash string) bool {
	return storedHash != "" && storedHash == HashRefreshToken(refreshToken)
}
