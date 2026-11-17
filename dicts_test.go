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
	"strings"
	"testing"

	"TravelManager/config"
	"TravelManager/database"
	"TravelManager/router"
)

// TestSystemDictsAndTeamLocation 系统字典（自定义团型 / 高德 Key）与团队地点持久化
func TestSystemDictsAndTeamLocation(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_dicts_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmp,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_dicts_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	admin := registerAdmin(t, r)

	// 默认字典
	resp := doJSON(t, r, "/api/system/dicts", "GET", nil, admin)
	mustOK(t, resp)
	for _, want := range []string{`"team_types"`, `"研学"`, `"团建"`, `"旅游团"`, `"expense_categories"`, `"门票"`, `"住宿"`, `"餐饮"`, `"景区内部消费"`, `"amap_key"`} {
		if !strings.Contains(string(resp.Body), want) {
			t.Fatalf("dicts missing %s: %s", want, string(resp.Body))
		}
	}

	// 管理员自定义团型 + 配置高德 Key
	body, _ := json.Marshal(map[string]string{
		"team_types": "研学,团建,康养,亲子", "expense_categories": "门票,住宿,餐饮,保险", "amap_key": "test-amap-key-123",
	})
	resp = doJSON(t, r, "/api/settings", "PUT", body, admin)
	mustOK(t, resp)
	resp = doJSON(t, r, "/api/system/dicts", "GET", nil, admin)
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"康养"`) ||
		!strings.Contains(string(resp.Body), `"保险"`) ||
		!strings.Contains(string(resp.Body), `"amap_key":"test-amap-key-123"`) {
		t.Fatalf("dicts not updated: %s", string(resp.Body))
	}

	// 创建带地点信息的团队
	agentID := createAgent(t, r, admin, "地图选点旅行社")
	body, _ = json.Marshal(map[string]any{
		"name": "沂蒙山康养团", "agent_id": agentID, "team_type": "康养",
		"location_name": "沂蒙山旅游区", "location_lng": 117.9556, "location_lat": 35.5242,
		"people_count": 30, "status": "booked",
	})
	resp = doJSON(t, r, "/api/teams", "POST", body, admin)
	mustOK(t, resp)
	var team struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &team)
	if team.Data.ID == 0 {
		t.Fatalf("team not created: %s", string(resp.Body))
	}

	// 详情应包含地点
	resp = doJSON(t, r, "/api/teams/"+strconv.FormatUint(uint64(team.Data.ID), 10), "GET", nil, admin)
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"location_name":"沂蒙山旅游区"`) ||
		!strings.Contains(string(resp.Body), `"location_lng":117.9556`) ||
		!strings.Contains(string(resp.Body), `"location_lat":35.5242`) {
		t.Fatalf("team location not persisted: %s", string(resp.Body))
	}

	// 补录自定义分类消费
	body, _ = json.Marshal(map[string]any{"category": "保险", "category_name": "保险", "description": "旅游意外险", "amount": 300})
	resp = doJSON(t, r, "/api/teams/"+strconv.FormatUint(uint64(team.Data.ID), 10)+"/expenses", "POST", body, admin)
	mustOK(t, resp)
	resp = doJSON(t, r, "/api/teams/"+strconv.FormatUint(uint64(team.Data.ID), 10), "GET", nil, admin)
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"category_name":"保险"`) {
		t.Fatalf("custom expense category not persisted: %s", string(resp.Body))
	}

	t.Log("dicts / team location / expense category test passed")
}

// TestTeamCreateParamErrors 团队创建：YYYY-MM-DD 日期可创建；错误参数给出具体提示
func TestTeamCreateParamErrors(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_team_param_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmp,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_team_param_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	admin := registerAdmin(t, r)
	agentID := createAgent(t, r, admin, "参数校验旅行社")

	// 前端提交的 YYYY-MM-DD 日期应可正常创建
	body, _ := json.Marshal(map[string]any{
		"name": "日期格式团队", "agent_id": agentID, "people_count": 20, "status": "booked",
		"arrive_date": "2026-08-01", "leave_date": "2026-08-02",
	})
	resp := doJSON(t, r, "/api/teams", "POST", body, admin)
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"arrive_date":"2026-08-01T00:00:00+08:00"`) {
		t.Fatalf("team created with dates unexpected: %s", string(resp.Body))
	}

	// 非法日期应给出具体字段提示
	body, _ = json.Marshal(map[string]any{"name": "坏日期", "agent_id": agentID, "arrive_date": "2026-13-99"})
	resp = doJSON(t, r, "/api/teams", "POST", body, admin)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(resp.Body), "抵达日期") {
		t.Fatalf("bad date should give specific error, got %d %s", resp.StatusCode, string(resp.Body))
	}

	// 非法电话应给出具体字段提示
	body, _ = json.Marshal(map[string]any{"name": "坏电话", "agent_id": agentID, "guide_phone": "abc123"})
	resp = doJSON(t, r, "/api/teams", "POST", body, admin)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(resp.Body), "导游电话格式不正确") {
		t.Fatalf("bad phone should give specific error, got %d %s", resp.StatusCode, string(resp.Body))
	}

	// 缺少团队名称
	body, _ = json.Marshal(map[string]any{"agent_id": agentID})
	resp = doJSON(t, r, "/api/teams", "POST", body, admin)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(resp.Body), "团队名称不能为空") {
		t.Fatalf("missing name should give specific error, got %d %s", resp.StatusCode, string(resp.Body))
	}

	t.Log("team create param error test passed")
}
