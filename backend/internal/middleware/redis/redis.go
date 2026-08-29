package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"feedsystem_video_go/internal/config"
	"strconv"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewFromEnv(cfg *config.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + strconv.Itoa(cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &Client{rdb: rdb}, nil
}

func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Ping(ctx).Err()
}

func IsMiss(err error) bool {
	return err == redis.Nil
}

func randToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (c *Client) Lock(ctx context.Context, key string, ttl time.Duration) (token string, ok bool, err error) {
	if c == nil || c.rdb == nil {
		return "", false, nil
	}
	token, err = randToken(16)
	if err != nil {
		return "", false, err
	}
	ok, err = c.rdb.SetNX(ctx, key, token, ttl).Result()
	return token, ok, err
}

var unlockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
else
  return 0
end
`)

func (c *Client) Unlock(ctx context.Context, key string, token string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	_, err := unlockScript.Run(ctx, c.rdb, []string{key}, token).Result()
	return err
}

func (c *Client) IncrementWithExpire(ctx context.Context, key string, expire time.Duration) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, nil
	}
	count, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		err = c.rdb.Expire(ctx, key, expire).Err()
		if err != nil {
			return 0, err
		}
	}
	return count, nil
}

var tokenBucketScript = redis.NewScript(`
local tokens = tonumber(redis.call('HGET', KEYS[1], 'tokens') or '-1')
local last = tonumber(redis.call('HGET', KEYS[1], 'last') or '0')
local now = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local capacity = tonumber(ARGV[3])
local ttl = tonumber(ARGV[4])

if tokens < 0 then
	tokens = capacity
else
	tokens = math.min(tokens + (now - last) /1000 * rate, capacity)
end

redis.call('HSET', KEYS[1], 'tokens', tokens, 'last', now)
redis.call('PEXPIRE', KEYS[1], ttl)

if tokens < 1 then 
	return 0
end
redis.call('HSET', KEYS[1], 'tokens', tokens - 1)
return 1
`)

func (c *Client) TokenBucketAllow(ctx context.Context, key string, rate, capacity float64, ttl time.Duration) (bool, error) {
	if c == nil || c.rdb == nil {
		return true, nil
	}
	res, err := tokenBucketScript.Run(ctx, c.rdb, []string{key}, float64(time.Now().UnixMilli()), rate, capacity, int64(ttl/time.Millisecond)).Result()
	if err != nil {
		return false, err
	}
	return res.(int64) == 1, nil
}
