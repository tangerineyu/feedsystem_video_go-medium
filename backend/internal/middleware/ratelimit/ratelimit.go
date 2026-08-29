package ratelimit

import (
	"bytes"
	"encoding/json"
	jwt "feedsystem_video_go/internal/middleware/jwt"
	rediscache "feedsystem_video_go/internal/middleware/redis"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type KeyFunc func(*gin.Context) (string, bool)

func Limit(
	cache *rediscache.Client,
	keyPrefix string,
	maxRequests int64,
	window time.Duration,
	keyFunc KeyFunc,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cache == nil || keyFunc == nil || maxRequests <= 0 || window <= 0 {
			c.Next()
			return
		}
		subject, ok := keyFunc(c)
		if !ok {
			c.Next()
			return
		}
		key := buildKey(keyPrefix, subject)
		count, err := cache.IncrementWithExpire(c.Request.Context(), key, window)
		if err != nil {
			c.Next()
			return
		}
		if count > maxRequests {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests",
			})
			return
		}
		c.Next()
	}
}

func buildKey(keyPrefix, subject string) string {
	keyPrefix = strings.TrimSpace(keyPrefix)
	if keyPrefix == "" {
		keyPrefix = "default"
	}
	return fmt.Sprintf("feedsystem:ratelimit:%s:%s", keyPrefix, strings.TrimSpace(subject))
}

// KeyByIPAndUsername 按 IP + 用户名组合限流：共享出口 IP 下不同账号各自计数互不影响，
// 同一 IP 对同一账号的暴力穷举仍被限制。
func KeyByIPAndUsername(c *gin.Context) (string, bool) {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		return "", false
	}
	username := extractUsername(c)
	if username == "" {
		return ip, true
	}
	return ip + "|" + username, true
}

// extractUsername 读取请求 body 中的 username，并恢复 body 供后续 handler 使用。
func extractUsername(c *gin.Context) string {
	body, err := c.GetRawData()
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	var req struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}
	return strings.TrimSpace(req.Username)
}

func KeyByAccount(c *gin.Context) (string, bool) {
	accountID, err := jwt.GetAccountID(c)
	if err != nil || accountID == 0 {
		return "", false
	}
	return strconv.FormatUint(uint64(accountID), 10), true
}

func TokenBucketLimit(
	cache *rediscache.Client,
	keyPrefix string,
	rate float64,
	capacity float64,
	ttl time.Duration,
	keyFunc KeyFunc,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cache == nil || keyFunc == nil || rate <= 0 || capacity <= 0 {
			c.Next()
			return
		}
		subject, ok := keyFunc(c)
		if !ok {
			c.Next()
			return
		}
		allowed, err := cache.TokenBucketAllow(c.Request.Context(), buildKey(keyPrefix, subject), rate, capacity, ttl)
		if err != nil {
			c.Next()
			return
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests",
			})
			return
		}
		c.Next()
	}
}
