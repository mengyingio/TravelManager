/**
 * 版权所有 © 云电工作室（https://www.ce-studio.cn）
 * 本项目由云电工作室成员共同创作编写
 * 以 MIT License 开源，详见项目根目录 LICENSE 文件
 */

package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"TravelManager/config"
	"TravelManager/database"
	"TravelManager/models"
	"TravelManager/router"
)

func TestDeleteUserRemovesLoginCredentialsAndPreservesAuditLog(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Port:              "0",
		DBPath:            filepath.Join(tmpDir, "users.db"),
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(tmpDir, "uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	adminToken := registerAdmin(t, r)

	createUser(t, r, adminToken, map[string]any{
		"username": "oauth_user", "password": "pass1234", "name": "第三方用户", "role": "team_leader",
	})
	var user models.User
	if err := database.DB.Where("username = ?", "oauth_user").First(&user).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if err := database.DB.Create(&models.UserOAuth{
		UserID: user.ID, Provider: "wechat", SocialUID: "wechat-delete-test",
	}).Error; err != nil {
		t.Fatalf("create oauth binding: %v", err)
	}
	if err := database.DB.Create(&models.PasskeyCredential{
		UserID: user.ID, Name: "测试密钥", CredentialID: "credential-delete-test", Data: `{}`,
	}).Error; err != nil {
		t.Fatalf("create passkey: %v", err)
	}
	if err := database.DB.Create(&models.OperationLog{
		UserID: user.ID, Username: user.Username, Action: "login", TargetType: "user", TargetID: user.ID,
		Detail: "待删除用户的历史日志",
	}).Error; err != nil {
		t.Fatalf("create audit log: %v", err)
	}

	resp := doJSON(t, r, fmt.Sprintf("/api/users/%d", user.ID), http.MethodDelete, nil, adminToken)
	mustOK(t, resp)

	var userCount, oauthCount, passkeyCount, logCount int64
	database.DB.Model(&models.User{}).Where("id = ?", user.ID).Count(&userCount)
	database.DB.Model(&models.UserOAuth{}).Where("user_id = ?", user.ID).Count(&oauthCount)
	database.DB.Model(&models.PasskeyCredential{}).Where("user_id = ?", user.ID).Count(&passkeyCount)
	database.DB.Model(&models.OperationLog{}).Where("user_id = ? AND detail = ?", user.ID, "待删除用户的历史日志").Count(&logCount)
	if userCount != 0 || oauthCount != 0 || passkeyCount != 0 {
		t.Fatalf("user credentials were not fully removed: user=%d oauth=%d passkey=%d", userCount, oauthCount, passkeyCount)
	}
	if logCount != 1 {
		t.Fatalf("historical audit log should be preserved, got %d", logCount)
	}
}
