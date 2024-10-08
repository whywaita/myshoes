package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/whywaita/myshoes/pkg/config"
)

// RateLimit is Docker Hub API rate limit
type RateLimit struct {
	Limit     int
	Remaining int
}

type tokenCache struct {
	expire time.Time
	token  string
}

var cacheMap = make(map[int]tokenCache, 1)

func getToken() (string, error) {
	url := "https://auth.docker.io/token?service=registry.docker.io&scope=repository:ratelimitpreview/test:pull"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	if config.Config.DockerHubCredential.Password != "" && config.Config.DockerHubCredential.Username != "" {
		req.SetBasicAuth(config.Config.DockerHubCredential.Username, config.Config.DockerHubCredential.Password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request token: %w", err)
	}
	if cache, ok := cacheMap[0]; ok && cache.expire.After(time.Now()) {
		return cache.token, nil
	}
	defer resp.Body.Close()
	byteArray, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	jsonMap := make(map[string]interface{})
	if err := json.Unmarshal(byteArray, &jsonMap); err != nil {
		return "", fmt.Errorf("unmarshal json: %w", err)
	}
	tokenString, ok := jsonMap["token"].(string)
	if !ok {
		return "", fmt.Errorf("tokenString is not string")
	}
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}
	exp, ok := token.Claims.(jwt.MapClaims)["exp"].(float64)
	if !ok {
		return "", fmt.Errorf("exp is not float64")
	}
	cacheMap[0] = tokenCache{
		expire: time.Unix(int64(exp), 0),
		token:  tokenString,
	}
	return tokenString, nil
}

// GetRateLimit get Docker Hub API rate limit
func GetRateLimit() (RateLimit, error) {
	token, err := getToken()
	if err != nil {
		return RateLimit{}, fmt.Errorf("get token: %w", err)
	}
	url := "https://registry-1.docker.io/v2/ratelimitpreview/test/manifests/latest"
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return RateLimit{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return RateLimit{}, fmt.Errorf("get rate limit: %w", err)
	}
	defer resp.Body.Close()
	limitHeader := resp.Header.Get("ratelimit-limit")
	if limitHeader == "" {
		return RateLimit{}, fmt.Errorf("not found ratelimit-limit header")
	}
	limit, err := strconv.Atoi(strings.Split(limitHeader, ";")[0])
	if err != nil {
		return RateLimit{}, fmt.Errorf("parse limit: %w", err)
	}
	remainingHeader := resp.Header.Get("ratelimit-remaining")
	if remainingHeader == "" {
		return RateLimit{}, fmt.Errorf("not found ratelimit-remaining header")

	}
	remaining, err := strconv.Atoi(strings.Split(remainingHeader, ";")[0])
	if err != nil {
		return RateLimit{}, fmt.Errorf("parse remaining: %w", err)
	}

	return RateLimit{
		Limit:     limit,
		Remaining: remaining,
	}, nil
}
