/**
 * 版权所有 © 云电工作室（https://www.ce-studio.cn）
 * 本项目由云电工作室成员共同创作编写
 * 以 MIT License 开源，详见项目根目录 LICENSE 文件
 */

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"TravelManager/config"
	"TravelManager/database"
	"TravelManager/models"
	"TravelManager/router"
)

func TestGuideProfilesAndTeamSnapshots(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_guides_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port: "0", DBPath: tmp, JWTSecret: "test-secret", InitialAdminToken: "test-initial-admin-token",
		UploadDir: filepath.Join(os.TempDir(), "tm_guides_uploads"), TokenTTLHours: 1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	admin := registerAdmin(t, r)
	agentID := createAgent(t, r, admin, "导游测试旅行社")

	createUser(t, r, admin, map[string]any{
		"username": "guide_agent", "password": "pass1234", "name": "旅行社账号",
		"phone": "13800138031", "email": "guide-agent@example.com", "role": "agent_owner", "agent_id": agentID,
	})
	agentToken := loginToken(t, r, "guide_agent", "pass1234")

	profileBody, _ := json.Marshal(map[string]any{
		"name": "王导", "phone": "13900139031", "email": "wang-guide@example.com", "remark": "金牌导游", "status": "active",
	})
	resp := doJSON(t, r, "/api/guides", http.MethodPost, profileBody, admin)
	mustOK(t, resp)
	var created struct {
		Data models.GuideProfile `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &created); err != nil || created.Data.ID == 0 {
		t.Fatalf("decode created guide: %v %s", err, resp.Body)
	}
	guideID := created.Data.ID

	resp = doJSON(t, r, "/api/guides", http.MethodPost, profileBody, agentToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("agent owner should not create guide, got %d", resp.StatusCode)
	}
	resp = doJSON(t, r, "/api/guides", http.MethodGet, nil, agentToken)
	mustOK(t, resp)
	if !jsonContainsGuide(t, resp.Body, guideID) {
		t.Fatal("agent owner should list active guides")
	}

	teamBody, _ := json.Marshal(map[string]any{
		"name": "导游快照测试团", "agent_id": agentID, "guide_id": guideID,
		"guide_name": "伪造导游", "guide_phone": "13800000000", "guide_email": "fake@example.com", "status": "booked",
	})
	resp = doJSON(t, r, "/api/teams", http.MethodPost, teamBody, agentToken)
	mustOK(t, resp)
	var teamResp struct {
		Data models.Team `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &teamResp); err != nil {
		t.Fatalf("decode team: %v", err)
	}
	if teamResp.Data.GuideID == nil || *teamResp.Data.GuideID != guideID ||
		teamResp.Data.GuideName != "王导" || teamResp.Data.GuidePhone != "13900139031" || teamResp.Data.GuideEmail != "wang-guide@example.com" {
		t.Fatalf("guide snapshot not applied: %+v", teamResp.Data)
	}
	teamID := teamResp.Data.ID

	updatedBody, _ := json.Marshal(map[string]any{
		"name": "王导新资料", "phone": "13900139032", "email": "wang-new@example.com", "status": "active",
	})
	resp = doJSON(t, r, fmt.Sprintf("/api/guides/%d", guideID), http.MethodPut, updatedBody, admin)
	mustOK(t, resp)
	resp = doJSON(t, r, fmt.Sprintf("/api/teams/%d", teamID), http.MethodGet, nil, agentToken)
	mustOK(t, resp)
	if err := json.Unmarshal(resp.Body, &teamResp); err != nil {
		t.Fatalf("decode stored team: %v", err)
	}
	if teamResp.Data.GuideName != "王导" || teamResp.Data.GuidePhone != "13900139031" || teamResp.Data.GuideEmail != "wang-guide@example.com" {
		t.Fatalf("existing team snapshot changed with profile: %+v", teamResp.Data)
	}

	disabledBody, _ := json.Marshal(map[string]any{
		"name": "王导新资料", "phone": "13900139032", "email": "wang-new@example.com", "status": "disabled",
	})
	resp = doJSON(t, r, fmt.Sprintf("/api/guides/%d", guideID), http.MethodPut, disabledBody, admin)
	mustOK(t, resp)
	resp = doJSON(t, r, "/api/guides", http.MethodGet, nil, agentToken)
	mustOK(t, resp)
	if jsonContainsGuide(t, resp.Body, guideID) {
		t.Fatal("agent owner should not list disabled guides")
	}
	teamBody, _ = json.Marshal(map[string]any{
		"name": "不可分配停用导游", "agent_id": agentID, "guide_id": guideID, "status": "booked",
	})
	resp = doJSON(t, r, "/api/teams", http.MethodPost, teamBody, agentToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("disabled guide should not be assigned, got %d: %s", resp.StatusCode, resp.Body)
	}
}

func jsonContainsGuide(t *testing.T, body []byte, id uint) bool {
	t.Helper()
	var response struct {
		Data []models.GuideProfile `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode guide list: %v", err)
	}
	for _, guide := range response.Data {
		if guide.ID == id {
			return true
		}
	}
	return false
}
