package ratelimit

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestKeyByIPAndUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newCtx := func(body string) *gin.Context {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/account/login", strings.NewReader(body))
		return c
	}

	ipOnly := newCtx(`{"password":"x"}`)
	key, ok := KeyByIPAndUsername(ipOnly)
	if !ok || key != ipOnly.ClientIP() {
		t.Fatalf("空用户名应 fallback 到纯 IP, got key=%q ok=%v", key, ok)
	}

	alice1 := newCtx(`{"username":"alice","password":"x"}`)
	key1, ok1 := KeyByIPAndUsername(alice1)
	if !ok1 || key1 != alice1.ClientIP()+"|alice" {
		t.Fatalf("应生成 IP|username, got key=%q ok=%v", key1, ok1)
	}

	alice2 := newCtx(`{"username":"alice","password":"x"}`)
	key2, _ := KeyByIPAndUsername(alice2)
	if key2 != key1 {
		t.Fatalf("同一 IP 同一账号应共享计数, got %q vs %q", key1, key2)
	}

	bob := newCtx(`{"username":"bob","password":"x"}`)
	keyB, _ := KeyByIPAndUsername(bob)
	if keyB == key1 {
		t.Fatalf("同一 IP 不同账号应各自计数, got %q vs %q", key1, keyB)
	}
}

func TestExtractUsernameRestoresBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/account/login", strings.NewReader(`{"username":"alice","password":"x"}`))

	if got := extractUsername(c); got != "alice" {
		t.Fatalf("extractUsername = %q, want alice", got)
	}

	body, err := c.GetRawData()
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"username":"alice","password":"x"}` {
		t.Fatalf("body 未被恢复, got %q", string(body))
	}
}
