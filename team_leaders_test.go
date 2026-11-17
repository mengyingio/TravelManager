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

func TestTeamLeaderProfilesAndSnapshots(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_team_leaders_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port: "0", DBPath: tmp, JWTSecret: "test-secret", InitialAdminToken: "test-initial-admin-token",
		UploadDir: filepath.Join(os.TempDir(), "tm_team_leaders_uploads"), TokenTTLHours: 1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	admin := registerAdmin(t, r)
	agentID := createAgent(t, r, admin, "负责人测试旅行社")

	createUser(t, r, admin, map[string]any{
		"username": "leader_agent", "password": "pass1234", "name": "旅行社账号",
		"phone": "13800138021", "email": "agent-leader@example.com", "role": "agent_owner", "agent_id": agentID,
	})
	agentToken := loginToken(t, r, "leader_agent", "pass1234")

	profileBody, _ := json.Marshal(map[string]any{
		"name": "刘老师", "phone": "13900139021", "email": "liu-profile@example.com", "remark": "研学负责人", "status": "active",
	})
	resp := doJSON(t, r, "/api/team-leaders", http.MethodPost, profileBody, admin)
	mustOK(t, resp)
	var created struct {
		Data models.TeamLeaderProfile `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &created); err != nil || created.Data.ID == 0 {
		t.Fatalf("decode created leader: %v %s", err, resp.Body)
	}
	leaderID := created.Data.ID

	resp = doJSON(t, r, "/api/team-leaders", http.MethodPost, profileBody, agentToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("agent owner should not create leader, got %d", resp.StatusCode)
	}
	resp = doJSON(t, r, "/api/team-leaders", http.MethodGet, nil, agentToken)
	mustOK(t, resp)
	if !jsonContainsLeader(t, resp.Body, leaderID) {
		t.Fatal("agent owner should list active leaders")
	}

	teamBody, _ := json.Marshal(map[string]any{
		"name": "负责人快照测试团", "agent_id": agentID, "leader_id": leaderID,
		"leader_name": "伪造姓名", "leader_phone": "13800000000", "leader_email": "fake@example.com", "status": "booked",
	})
	resp = doJSON(t, r, "/api/teams", http.MethodPost, teamBody, agentToken)
	mustOK(t, resp)
	var teamResp struct {
		Data models.Team `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &teamResp); err != nil {
		t.Fatalf("decode team: %v", err)
	}
	if teamResp.Data.LeaderName != "刘老师" || teamResp.Data.LeaderPhone != "13900139021" || teamResp.Data.LeaderEmail != "liu-profile@example.com" {
		t.Fatalf("leader snapshot not applied: %+v", teamResp.Data)
	}
	teamID := teamResp.Data.ID

	updatedBody, _ := json.Marshal(map[string]any{
		"name": "刘老师新资料", "phone": "13900139022", "email": "liu-new@example.com", "status": "active",
	})
	resp = doJSON(t, r, fmt.Sprintf("/api/team-leaders/%d", leaderID), http.MethodPut, updatedBody, admin)
	mustOK(t, resp)
	resp = doJSON(t, r, fmt.Sprintf("/api/teams/%d", teamID), http.MethodGet, nil, agentToken)
	mustOK(t, resp)
	if err := json.Unmarshal(resp.Body, &teamResp); err != nil {
		t.Fatalf("decode stored team: %v", err)
	}
	if teamResp.Data.LeaderName != "刘老师" || teamResp.Data.LeaderPhone != "13900139021" || teamResp.Data.LeaderEmail != "liu-profile@example.com" {
		t.Fatalf("existing team snapshot changed with profile: %+v", teamResp.Data)
	}

	disabledBody, _ := json.Marshal(map[string]any{
		"name": "刘老师新资料", "phone": "13900139022", "email": "liu-new@example.com", "status": "disabled",
	})
	resp = doJSON(t, r, fmt.Sprintf("/api/team-leaders/%d", leaderID), http.MethodPut, disabledBody, admin)
	mustOK(t, resp)
	resp = doJSON(t, r, "/api/team-leaders", http.MethodGet, nil, agentToken)
	mustOK(t, resp)
	if jsonContainsLeader(t, resp.Body, leaderID) {
		t.Fatal("agent owner should not list disabled leaders")
	}
	teamBody, _ = json.Marshal(map[string]any{
		"name": "不可分配停用负责人", "agent_id": agentID, "leader_id": leaderID, "status": "booked",
	})
	resp = doJSON(t, r, "/api/teams", http.MethodPost, teamBody, agentToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("disabled leader should not be assigned, got %d: %s", resp.StatusCode, resp.Body)
	}
}

func jsonContainsLeader(t *testing.T, body []byte, id uint) bool {
	t.Helper()
	var response struct {
		Data []models.TeamLeaderProfile `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode leader list: %v", err)
	}
	for _, leader := range response.Data {
		if leader.ID == id {
			return true
		}
	}
	return false
}
