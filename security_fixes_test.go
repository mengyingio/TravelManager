/**
 * 版权所有 © 云电工作室（https://www.ce-studio.cn）
 * 本项目由云电工作室成员共同创作编写
 * 以 MIT License 开源，详见项目根目录 LICENSE 文件
 */

package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"TravelManager/config"
	"TravelManager/database"
	"TravelManager/router"
)

func TestSecurityBoundaries(t *testing.T) {
	cfg := &config.Config{
		Port: "0", DBPath: filepath.Join(t.TempDir(), "security.db"), JWTSecret: "test-secret",
		InitialAdminToken: "test-initial-admin-token", UploadDir: filepath.Join(t.TempDir(), "uploads"), TokenTTLHours: 1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	superToken := registerAdmin(t, r)

	createUser(t, r, superToken, map[string]any{
		"username": "ordinary_admin", "password": "pass1234", "name": "普通管理员", "role": "admin",
	})
	adminToken := loginToken(t, r, "ordinary_admin", "pass1234")
	resp := doJSON(t, r, "/api/users/1/reset-password", http.MethodPost, []byte(`{"password":"changed123"}`), adminToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("administrator must not reset super-admin password, got %d: %s", resp.StatusCode, resp.Body)
	}

	createUser(t, r, superToken, map[string]any{
		"username": "leader", "password": "pass1234", "name": "负责人", "role": "team_leader", "phone": "13800138001", "email": "leader@example.com",
	})
	leaderToken := loginToken(t, r, "leader", "pass1234")
	body, _ := json.Marshal(map[string]string{
		"name": "负责人", "nickname": "", "phone": "13800138002", "email": "other@example.com",
	})
	resp = doJSON(t, r, "/api/users/me", http.MethodPut, body, leaderToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("leader must not change scope contacts, got %d: %s", resp.StatusCode, resp.Body)
	}

	teamID := createTeam(t, r, superToken, 0, "归档保护测试团", "", "")
	teamPath := "/api/teams/" + strconv.FormatUint(uint64(teamID), 10)
	resp = doJSON(t, r, teamPath+"/archive", http.MethodPost, nil, superToken)
	mustOK(t, resp)
	resp = doJSON(t, r, teamPath, http.MethodDelete, nil, superToken)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("archived team must not be deleted, got %d: %s", resp.StatusCode, resp.Body)
	}

	resp = doJSON(t, r, "/api/settings", http.MethodPut, []byte(`{"smtp_password":"mail-secret"}`), superToken)
	mustOK(t, resp)
	resp = doJSON(t, r, "/api/settings", http.MethodGet, nil, superToken)
	mustOK(t, resp)
	if strings.Contains(string(resp.Body), "mail-secret") || !strings.Contains(string(resp.Body), "********") {
		t.Fatalf("settings response must mask stored secrets: %s", resp.Body)
	}
}

func TestInitialAdminTokenRequired(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Port: "0", DBPath: filepath.Join(tmpDir, "initial-admin.db"), JWTSecret: "test-secret",
		UploadDir: filepath.Join(tmpDir, "uploads"), TokenTTLHours: 1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	body := []byte(`{"username":"admin","password":"pass1234"}`)
	resp := doJSON(t, router.New(cfg), "/api/auth/register", http.MethodPost, body, "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("missing initial token must return 503, got %d: %s", resp.StatusCode, resp.Body)
	}

	cfg.InitialAdminToken = "test-initial-admin-token"
	resp = doJSON(t, router.New(cfg), "/api/auth/register", http.MethodPost,
		[]byte(`{"username":"admin","password":"pass1234","setup_token":"wrong"}`), "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong initial token must return 403, got %d: %s", resp.StatusCode, resp.Body)
	}
	resp = doJSON(t, router.New(cfg), "/api/auth/register", http.MethodPost,
		[]byte(`{"username":"admin","password":"pass1234","setup_token":"test-initial-admin-token"}`), "")
	mustOK(t, resp)
}
