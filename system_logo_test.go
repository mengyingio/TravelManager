/**
 * 版权所有 © 云电工作室（https://www.ce-studio.cn）
 * 本项目由云电工作室成员共同创作编写
 * 以 MIT License 开源，详见项目根目录 LICENSE 文件
 */

package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"TravelManager/config"
	"TravelManager/database"
	"TravelManager/router"
)

func TestSystemLogoUploadAndReset(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{
		Port: "0", DBPath: filepath.Join(tmp, "logo.db"), JWTSecret: "test-secret", InitialAdminToken: "test-initial-admin-token",
		UploadDir: filepath.Join(tmp, "uploads"), TokenTTLHours: 1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)

	registerBody, _ := json.Marshal(map[string]string{"username": "logo-admin", "password": "pass1234"})
	registerResp := doJSON(t, r, "/api/auth/register", http.MethodPost, registerBody, "")
	mustOK(t, registerResp)
	var registered struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(registerResp.Body, &registered); err != nil || registered.Data.Token == "" {
		t.Fatalf("missing admin token: %s", string(registerResp.Body))
	}

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, img); err != nil {
		t.Fatal(err)
	}
	body, contentType := buildMultipart(t, "file", "brand.png", imageData.String())
	uploadResp := doReq(t, r, "/api/settings/logo", http.MethodPost, body, contentType, registered.Data.Token)
	mustOK(t, uploadResp)
	var uploaded struct {
		Data struct {
			LogoURL string `json:"logo_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(uploadResp.Body, &uploaded); err != nil || !strings.HasPrefix(uploaded.Data.LogoURL, "/uploads/branding/logo-") {
		t.Fatalf("unexpected logo response: %s", string(uploadResp.Body))
	}

	logoResp := doJSON(t, r, uploaded.Data.LogoURL, http.MethodGet, nil, "")
	if logoResp.StatusCode != http.StatusOK || !bytes.Equal(logoResp.Body, imageData.Bytes()) {
		t.Fatalf("public logo unavailable: %d", logoResp.StatusCode)
	}
	infoResp := doJSON(t, r, "/api/system/info", http.MethodGet, nil, "")
	if !bytes.Contains(infoResp.Body, []byte(uploaded.Data.LogoURL)) {
		t.Fatalf("system info missing logo: %s", string(infoResp.Body))
	}

	badBody, badType := buildMultipart(t, "file", "unsafe.svg", `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	badResp := doReq(t, r, "/api/settings/logo", http.MethodPost, badBody, badType, registered.Data.Token)
	if badResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("SVG must be rejected: %d %s", badResp.StatusCode, string(badResp.Body))
	}

	resetResp := doJSON(t, r, "/api/settings/logo", http.MethodDelete, nil, registered.Data.Token)
	mustOK(t, resetResp)
	if _, err := os.Stat(filepath.Join(cfg.UploadDir, "branding", filepath.Base(uploaded.Data.LogoURL))); !os.IsNotExist(err) {
		t.Fatalf("old logo should be removed, stat err: %v", err)
	}
	infoResp = doJSON(t, r, "/api/system/info", http.MethodGet, nil, "")
	if !bytes.Contains(infoResp.Body, []byte(`"logo_url":""`)) {
		t.Fatalf("logo should be reset: %s", string(infoResp.Body))
	}
}
