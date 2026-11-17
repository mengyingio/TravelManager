/**
 * 版权所有 © 云电工作室（https://www.ce-studio.cn）
 * 本项目由云电工作室成员共同创作编写
 * 以 MIT License 开源，详见项目根目录 LICENSE 文件
 */

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"TravelManager/config"
	"TravelManager/database"
	"TravelManager/router"
)

func TestBusinessTeamCanRemainUnassociated(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_business_team_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmp,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_business_team_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	admin := registerAdmin(t, r)
	agentID := createAgent(t, r, admin, "远航旅行社")
	date := fmt.Sprintf("%d-09-11", time.Now().Year())

	businessID := createTeamForAgentRule(t, r, admin, map[string]any{
		"name": "远航旅行社商务接待团", "team_type": "商务团队",
		"arrive_date": date, "people_count": 8, "status": "booked",
	})
	assertTeamAgent(t, businessID, false, 0)
	resp := doJSON(t, r, fmt.Sprintf("/api/teams/%d", businessID), http.MethodGet, nil, admin)
	mustOK(t, resp)
	resp = doJSON(t, r, "/api/teams", http.MethodGet, nil, admin)
	mustOK(t, resp)

	tourismID := createTeamForAgentRule(t, r, admin, map[string]any{
		"name": "远航旅行社旅游团", "team_type": "旅游团",
		"arrive_date": date, "people_count": 20, "status": "booked",
	})
	assertTeamAgent(t, tourismID, true, agentID)

	explicitID := createTeamForAgentRule(t, r, admin, map[string]any{
		"name": "商务团队（指定旅行社）", "team_type": "商务团队", "agent_id": agentID,
		"arrive_date": date, "people_count": 6, "status": "booked",
	})
	assertTeamAgent(t, explicitID, true, agentID)

	body, _ := json.Marshal(map[string]any{
		"name": "远航旅行社商务接待团", "team_type": "商务团队", "agent_id": nil,
		"arrive_date": date, "people_count": 8, "status": "booked",
	})
	resp = doJSON(t, r, fmt.Sprintf("/api/teams/%d", explicitID), http.MethodPut, body, admin)
	mustOK(t, resp)
	assertTeamAgent(t, explicitID, false, 0)

	resp = doJSON(t, r, "/api/stats/summary", http.MethodGet, nil, admin)
	mustOK(t, resp)
	var stats struct {
		Data struct {
			TotalTeams int64 `json:"total_teams"`
			ByAgent    []struct {
				AgentID uint  `json:"agent_id"`
				Count   int64 `json:"count"`
			} `json:"by_agent"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.Data.TotalTeams != 3 {
		t.Fatalf("total teams want 3, got %d", stats.Data.TotalTeams)
	}
	if len(stats.Data.ByAgent) != 1 || stats.Data.ByAgent[0].AgentID != agentID || stats.Data.ByAgent[0].Count != 1 {
		t.Fatalf("unassociated business team must not appear in travel-agent revenue: %+v", stats.Data.ByAgent)
	}
}

func createTeamForAgentRule(t *testing.T, r http.Handler, token string, values map[string]any) uint {
	t.Helper()
	body, _ := json.Marshal(values)
	resp := doJSON(t, r, "/api/teams", http.MethodPost, body, token)
	mustOK(t, resp)
	var result struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		t.Fatalf("decode team: %v", err)
	}
	return result.Data.ID
}

func assertTeamAgent(t *testing.T, teamID uint, wantValid bool, wantID uint) {
	t.Helper()
	var agentID sql.NullInt64
	if err := database.DB.Raw("SELECT agent_id FROM teams WHERE id = ?", teamID).Row().Scan(&agentID); err != nil {
		t.Fatalf("read team agent: %v", err)
	}
	if agentID.Valid != wantValid || (wantValid && uint(agentID.Int64) != wantID) {
		t.Fatalf("team %d agent want valid=%v id=%d, got valid=%v id=%d", teamID, wantValid, wantID, agentID.Valid, agentID.Int64)
	}
}
