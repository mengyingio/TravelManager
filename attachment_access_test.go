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
	"strconv"
	"testing"

	"TravelManager/config"
	"TravelManager/database"
	"TravelManager/router"
)

// TestTeamAttachmentPrivateAccess verifies that archived team files stay private
// while authorized users can still retrieve the original uploaded content.
func TestTeamAttachmentPrivateAccess(t *testing.T) {
	tmpDB := filepath.Join(os.TempDir(), "tm_attachment_access_test.db")
	_ = os.Remove(tmpDB)
	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmpDB,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_attachment_access_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	admin := registerAdmin(t, r)
	agentID := createAgent(t, r, admin, "附件访问旅行社")

	body, _ := json.Marshal(map[string]any{"name": "附件访问测试团", "agent_id": agentID, "status": "booked"})
	resp := doJSON(t, r, "/api/teams", http.MethodPost, body, admin)
	mustOK(t, resp)
	var team struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &team); err != nil {
		t.Fatalf("decode team: %v", err)
	}
	teamID := strconv.FormatUint(uint64(team.Data.ID), 10)

	form, contentType := buildMultipart(t, "file", "fujian.jpg", "private-photo-content")
	resp = doReq(t, r, "/api/teams/"+teamID+"/attachments", http.MethodPost, form, contentType, admin)
	mustOK(t, resp)
	var attachment struct {
		Data struct {
			ID       uint   `json:"id"`
			FilePath string `json:"file_path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &attachment); err != nil {
		t.Fatalf("decode attachment: %v", err)
	}

	// The file path must not be accessible without authentication.
	resp = doJSON(t, r, attachment.Data.FilePath, http.MethodGet, nil, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("private attachment should not be public, got %d: %s", resp.StatusCode, string(resp.Body))
	}

	resp = doJSON(t, r, "/api/teams/"+teamID+"/attachments/"+strconv.FormatUint(uint64(attachment.Data.ID), 10)+"/download", http.MethodGet, nil, admin)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authorized attachment retrieval failed: %d %s", resp.StatusCode, string(resp.Body))
	}
	if string(resp.Body) != "private-photo-content" {
		t.Fatalf("unexpected attachment content: %q", string(resp.Body))
	}
}
