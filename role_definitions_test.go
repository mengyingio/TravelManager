/**
 * 版权所有 © 云电工作室（https://www.ce-studio.cn）
 * 本项目由云电工作室成员共同创作编写
 * 以 MIT License 开源，详见项目根目录 LICENSE 文件
 */

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"TravelManager/config"
	"TravelManager/database"
	"TravelManager/models"
	"TravelManager/router"
)

func TestEditBuiltInRolePermissions(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_builtin_role_edit_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port: "0", DBPath: tmp, JWTSecret: "test-secret", InitialAdminToken: "test-initial-admin-token",
		UploadDir: filepath.Join(os.TempDir(), "tm_builtin_role_edit_uploads"), TokenTTLHours: 1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	superToken := registerAdmin(t, r)

	createUser(t, r, superToken, map[string]any{
		"username": "limited_admin", "password": "pass1234", "name": "受限管理员", "role": "admin",
	})
	adminToken := loginToken(t, r, "limited_admin", "pass1234")

	body, _ := json.Marshal(map[string]any{
		"key": "admin", "name": "管理员", "permissions": []string{"contracts.view"}, "data_scope": "none",
	})
	resp := doJSON(t, r, "/api/role-definitions", http.MethodPost, body, superToken)
	mustOK(t, resp)

	resp = doJSON(t, r, "/api/contracts", http.MethodGet, nil, adminToken)
	mustOK(t, resp)
	resp = doJSON(t, r, "/api/teams", http.MethodGet, nil, adminToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("edited admin role should not view teams, got %d: %s", resp.StatusCode, string(resp.Body))
	}

	body, _ = json.Marshal(map[string]any{
		"key": "super_admin", "name": "超级管理员", "permissions": []string{}, "data_scope": "none",
	})
	resp = doJSON(t, r, "/api/role-definitions", http.MethodPost, body, superToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("super admin role must remain immutable, got %d: %s", resp.StatusCode, string(resp.Body))
	}
}

func TestLegacyRolePermissionsMigrateWhenAutoMigrateSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Port: "0", DBPath: filepath.Join(tmpDir, "roles.db"), JWTSecret: "test-secret", InitialAdminToken: "test-initial-admin-token",
		UploadDir: filepath.Join(tmpDir, "uploads"), TokenTTLHours: 1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}

	legacy := `["dashboard.view","agents.view","agents.manage","teams.view","teams.manage"]`
	if err := database.DB.Model(&models.RoleDefinition{}).
		Where("role_key = ?", models.RoleAgentOwner).
		Update("permissions", legacy).Error; err != nil {
		t.Fatalf("write legacy permissions: %v", err)
	}

	skipCfg := *cfg
	skipCfg.SkipAutoMigrate = true
	if err := database.Init(&skipCfg); err != nil {
		t.Fatalf("init without schema migration: %v", err)
	}

	var role models.RoleDefinition
	if err := database.DB.Where("role_key = ?", models.RoleAgentOwner).First(&role).Error; err != nil {
		t.Fatalf("load migrated role: %v", err)
	}
	var permissions []string
	if err := json.Unmarshal([]byte(role.Permissions), &permissions); err != nil {
		t.Fatalf("decode migrated permissions: %v", err)
	}
	has := func(target string) bool {
		for _, permission := range permissions {
			if permission == target {
				return true
			}
		}
		return false
	}
	if !has("agents.edit") || !has("teams.create_delete") || has("agents.manage") {
		t.Fatalf("legacy permissions were not migrated: %v", permissions)
	}
}
