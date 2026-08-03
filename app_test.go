package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"squash-it/internal/app"
	"squash-it/internal/cache"
	"squash-it/internal/config"
	"squash-it/internal/db"
	"squash-it/internal/filter"
	"squash-it/internal/hash"
	"squash-it/internal/rate"
	"squash-it/internal/router"
	"strings"
	"testing"
)

var testHasher = hash.NewMurmurHash(6)

// Setup fresh app for each test
func setupTestApp(t *testing.T) http.Handler {
	cfg := config.Load()
	cfg.DBName = ":memory:"
	database := db.NewSQLite(cfg.DBName)
	pipeline := cache.NewLRUCache(100)
	bloom := filter.NewBloom(1000)

	limiter := rate.NewUserTokenBucket(cfg.RatePerMinute, cfg.RateBurst, cfg.RateCleanupInterval)

	r := router.NewRouter(router.WithHostPorts(cfg.Port))
	app.New(t.Context(), database, pipeline, bloom, testHasher, r, limiter)

	return r
}

func setupEncode(r http.Handler, input string) *httptest.ResponseRecorder {
	reqBody := []byte(`{"long_url":"` + input + `"}`)
	return sendRequestGetRecorder(r, http.MethodPost, "/encode", reqBody, nil)
}

func setupDecode(r http.Handler, input string) *httptest.ResponseRecorder {
	reqBody := []byte(`{"path_hash":"` + input + `"}`)
	return sendRequestGetRecorder(r, http.MethodPost, "/decode", reqBody, nil)
}

func sendRequestGetRecorder(r http.Handler, method, target string, payload []byte, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")

	if headers != nil {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	recorder := httptest.NewRecorder()

	r.ServeHTTP(recorder, req)

	return recorder

}

func TestFunctional_EncodeEndpoint(t *testing.T) {
	r := setupTestApp(t)

	input := "https://example.com/very-long-url-to-squash"

	encodeRec := setupEncode(r, input)

	if encodeRec.Code != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d. Body: %s", http.StatusOK, encodeRec.Code, encodeRec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(encodeRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if resp["short_url"] == "" {
		t.Errorf("Expected short_url in response, got none")
	}
}

func TestFunctional_EncodeDecodeFlow(t *testing.T) {
	r := setupTestApp(t)

	longURL := "https://example.com/very-long-url-to-squash"
	// /encode flow to get a hash
	encodeRec := setupEncode(r, longURL)
	var encodeResp map[string]string
	_ = json.Unmarshal(encodeRec.Body.Bytes(), &encodeResp)
	parts := strings.Split(encodeResp["short_url"], "/")
	h := parts[len(parts)-1]

	if h == "" {
		t.Fatalf("Failed to retrieve hash from encode response: %s", encodeRec.Body.String())
	}

	// /decode flow to get long_url
	decodeRec := setupDecode(r, h)
	var decodeResp map[string]string
	_ = json.Unmarshal(decodeRec.Body.Bytes(), &decodeResp)
	fmt.Println(decodeResp)
	if decodeResp["long_url"] != longURL {
		t.Fatalf("Expected long_url to be %s, got %s", longURL, decodeResp["long_url"])
	}
}

func TestFunctional_EncodeInvalidURL(t *testing.T) {
	r := setupTestApp(t)
	longURL := "malicious://example.com/very-long-url-to-squash"
	encodeRec := setupEncode(r, longURL)
	if encodeRec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status %d, got %d. Body: %s", http.StatusBadRequest, encodeRec.Code, encodeRec.Body.String())
	}
}

func TestFunctional_DecodeWithInvalidHash(t *testing.T) {
	r := setupTestApp(t)
	hash := "invalid-hash"
	decodeRec := setupDecode(r, hash)
	if decodeRec.Code != http.StatusNotFound {
		t.Fatalf("Excpected status %d, got %d", http.StatusFound, decodeRec.Code)
	}
}
