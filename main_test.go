/**
 * 版权所有 © 云电工作室（https://www.ce-studio.cn）
 * 本项目由云电工作室成员共同创作编写
 * 以 MIT License 开源，详见项目根目录 LICENSE 文件
 */

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"TravelManager/config"
	"TravelManager/database"
	"TravelManager/router"
)

type apiResp struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// TestAPISmoke 进程内冒烟测试：登录 → 建旅行社 → 建团队 → 加消费 → 统计 → 导出
func TestAPISmoke(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_smoke_test.db")
	_ = os.Remove(tmp)

	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmp,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_smoke_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)

	// 0. 公开系统信息（无需登录）
	{
		resp := doJSON(t, r, "/api/system/info", "GET", nil, "")
		mustOK(t, resp)
		if !bytes.Contains(resp.Body, []byte(`"name"`)) || !bytes.Contains(resp.Body, []byte("团队档案管理系统")) {
			t.Fatalf("system info unexpected: %s", string(resp.Body))
		}
		if !bytes.Contains(resp.Body, []byte(`"initialized":false`)) ||
			!bytes.Contains(resp.Body, []byte(`"oauth_enabled":false`)) {
			t.Fatalf("system info defaults unexpected: %s", string(resp.Body))
		}
	}

	// 1. 注册首个用户（自动成为超级管理员）并直接登录
	var token string
	{
		body, _ := json.Marshal(map[string]any{
			"username": "admin", "password": "pass1234", "name": "管理员",
			"phone": "13800138000", "email": "admin@example.com",
		})
		resp := doJSON(t, r, "/api/auth/register", "POST", body, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("register first user failed: %d %s", resp.StatusCode, string(resp.Body))
		}
		var r struct {
			Data struct {
				Token string `json:"token"`
				User  struct {
					Role string `json:"role"`
				} `json:"user"`
			} `json:"data"`
		}
		json.Unmarshal(resp.Body, &r)
		token = r.Data.Token
		if token == "" {
			t.Fatal("empty token")
		}
		if r.Data.User.Role != "super_admin" {
			t.Fatalf("first registered user should be super_admin, got %s", r.Data.User.Role)
		}
	}

	// 2. 新增旅行社
	var agentID uint
	{
		body, _ := json.Marshal(map[string]string{"name": "临沂阳光旅行社", "boss_name": "王总", "planner_name": "小李"})
		resp := doJSON(t, r, "/api/agents", "POST", body, token)
		mustOK(t, resp)
		var r struct {
			Data struct {
				ID uint `json:"id"`
			} `json:"data"`
		}
		json.Unmarshal(resp.Body, &r)
		agentID = r.Data.ID
	}

	// 3. 新增团队预订单
	var teamID uint
	{
		body, _ := json.Marshal(map[string]any{
			"name": "临沂50人研学团", "agent_id": agentID, "guide_name": "张导",
			"source_region": "临沂", "team_type": "研学", "people_count": 50,
			"leader_name": "刘老师", "status": "booked",
			"arrive_date": fmt.Sprintf("%d-01-10", time.Now().Year()),
			"leave_date":  fmt.Sprintf("%d-01-12", time.Now().Year()),
		})
		resp := doJSON(t, r, "/api/teams", "POST", body, token)
		mustOK(t, resp)
		var r struct {
			Data struct {
				ID uint `json:"id"`
			} `json:"data"`
		}
		json.Unmarshal(resp.Body, &r)
		teamID = r.Data.ID
	}

	// 4. 补录消费明细（门票 + 住宿）
	expenses := []map[string]any{
		{"category": "ticket", "category_name": "门票", "description": "成人票50张", "amount": 3000},
		{"category": "hotel", "category_name": "住宿", "description": "天园民宿15间", "amount": 4500},
	}
	for _, e := range expenses {
		body, _ := json.Marshal(e)
		resp := doJSON(t, r, fmt.Sprintf("/api/teams/%d/expenses", teamID), "POST", body, token)
		mustOK(t, resp)
	}

	// 5. 归档并导出 Word 简报
	{
		resp := doJSON(t, r, fmt.Sprintf("/api/teams/%d/archive", teamID), "POST", nil, token)
		mustOK(t, resp)
		resp = doJSON(t, r, fmt.Sprintf("/api/teams/%d/export", teamID), "GET", nil, token)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("export failed: %d %s", resp.StatusCode, string(resp.Body))
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
			t.Fatalf("unexpected content type: %s", ct)
		}
		if len(resp.Body) < 1000 {
			t.Fatalf("docx too small: %d", len(resp.Body))
		}
	}

	// 6. 团队详情校验总消费自动核算
	{
		resp := doJSON(t, r, fmt.Sprintf("/api/teams/%d", teamID), "GET", nil, token)
		mustOK(t, resp)
		var r struct {
			Data struct {
				TotalAmount float64 `json:"total_amount"`
				Status      string  `json:"status"`
			} `json:"data"`
		}
		json.Unmarshal(resp.Body, &r)
		if r.Data.TotalAmount != 7500 {
			t.Fatalf("total amount want 7500, got %v", r.Data.TotalAmount)
		}
		if r.Data.Status != "archived" {
			t.Fatalf("status want archived, got %s", r.Data.Status)
		}
	}

	// 7. 统计汇总
	{
		resp := doJSON(t, r, "/api/stats/summary", "GET", nil, token)
		mustOK(t, resp)
		if !bytes.Contains(resp.Body, []byte("total_teams")) {
			t.Fatalf("stats missing total_teams: %s", string(resp.Body))
		}
		var stats struct {
			Data struct {
				ByAgent []struct {
					AgentName string  `json:"agent_name"`
					Count     int64   `json:"count"`
					Amount    float64 `json:"amount"`
				} `json:"by_agent"`
			} `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &stats); err != nil || len(stats.Data.ByAgent) != 1 {
			t.Fatalf("stats by_agent unexpected: %s", string(resp.Body))
		}
		row := stats.Data.ByAgent[0]
		if row.AgentName != "临沂阳光旅行社" || row.Count != 1 || row.Amount != 7500 {
			t.Fatalf("stats by_agent want agent/1/7500, got %+v", row)
		}

		currentYear := time.Now().Year()
		resp = doJSON(t, r, fmt.Sprintf("/api/stats/summary?start_date=%d-01-10&end_date=%d-01-10", currentYear, currentYear), "GET", nil, token)
		mustOK(t, resp)
		var dateStats struct {
			Data struct {
				StartDate  string `json:"start_date"`
				EndDate    string `json:"end_date"`
				TotalTeams int64  `json:"total_teams"`
			} `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &dateStats); err != nil {
			t.Fatalf("decode date range stats: %v", err)
		}
		if dateStats.Data.TotalTeams != 1 || dateStats.Data.StartDate != fmt.Sprintf("%d-01-10", currentYear) || dateStats.Data.EndDate != fmt.Sprintf("%d-01-10", currentYear) {
			t.Fatalf("date range stats unexpected: %s", string(resp.Body))
		}

		resp = doJSON(t, r, "/api/stats/summary?start_date=2026-02-01&end_date=2026-01-01", "GET", nil, token)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("invalid date range should return 400, got %d: %s", resp.StatusCode, string(resp.Body))
		}
	}

	// 8. 操作日志应包含登录/新增/归档记录
	{
		resp := doJSON(t, r, "/api/logs?page_size=50", "GET", nil, token)
		mustOK(t, resp)
		if !bytes.Contains(resp.Body, []byte("用户登录")) {
			t.Fatalf("logs missing login record: %s", string(resp.Body))
		}
	}

	// 9. 系统名称自定义并公开生效
	{
		body, _ := json.Marshal(map[string]string{"system_name": "通用团队归档系统"})
		resp := doJSON(t, r, "/api/settings", "PUT", body, token)
		mustOK(t, resp)
		resp = doJSON(t, r, "/api/system/info", "GET", nil, "")
		mustOK(t, resp)
		if !bytes.Contains(resp.Body, []byte("通用团队归档系统")) {
			t.Fatalf("custom system name not applied: %s", string(resp.Body))
		}
	}

	// 10. 手机号格式校验：非法号码应被拒绝
	{
		body, _ := json.Marshal(map[string]string{"name": "坏号码旅行社", "boss_phone": "12345"})
		resp := doJSON(t, r, "/api/agents", "POST", body, token)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("invalid phone should be rejected: %d %s", resp.StatusCode, string(resp.Body))
		}
	}

	// 11. 旅行社 CSV 导入（合法行导入、非法手机号行跳过）
	{
		csvData := "\xEF\xBB\xBF公司名称,老板电话,老板邮箱,计调电话,计调邮箱\n" +
			"导入测试旅行社,13800138000,ok@example.com,010-12345678,plan@example.com\n" +
			"坏电话旅行社,12345,ok@example.com,,\n"
		body, ct := buildMultipart(t, "file", "agents.csv", csvData)
		resp := doReq(t, r, "/api/agents/import", "POST", body, ct, token)
		mustOK(t, resp)
		var ir struct {
			Data struct {
				Imported int `json:"imported"`
				Skipped  int `json:"skipped"`
			} `json:"data"`
		}
		json.Unmarshal(resp.Body, &ir)
		if ir.Data.Imported != 1 || ir.Data.Skipped != 1 {
			t.Fatalf("agent import want imported=1 skipped=1, got %+v", ir.Data)
		}
	}

	// 12. 团队 CSV 导入（日期 / 状态 / 旅行社名称匹配）
	{
		csvData := "\xEF\xBB\xBF团队名称,所属旅行社,导游,导游电话,客源地,团型,抵达日期,离开日期,人数,带队负责人,负责人电话,状态,备注\n" +
			"导入研学团,导入测试旅行社,王导,13900139000,临沂,研学,2026-08-01,2026-08-02,50,刘老师,13800138000,已归档,备注内容\n"
		body, ct := buildMultipart(t, "file", "teams.csv", csvData)
		resp := doReq(t, r, "/api/teams/import", "POST", body, ct, token)
		mustOK(t, resp)
		if !bytes.Contains(resp.Body, []byte(`"imported":1`)) {
			t.Fatalf("team import failed: %s", string(resp.Body))
		}
	}

	// 13. 导出 CSV：BOM + 表头
	{
		for _, path := range []string{"/api/agents/export", "/api/contracts/export", "/api/teams/export"} {
			resp := doJSON(t, r, path, "GET", nil, token)
			if resp.StatusCode != http.StatusOK || !bytes.HasPrefix(resp.Body, []byte("\xEF\xBB\xBF")) {
				t.Fatalf("export %s failed: %d", path, resp.StatusCode)
			}
		}
	}

	t.Log("smoke test passed")
}

// TestSPAServe 验证前端构建产物托管与 SPA 路由回退
func TestSPAServe(t *testing.T) {
	index := filepath.Join("web", "dist", "index.html")
	if _, err := os.Stat(index); err != nil {
		t.Skip("web/dist 未构建，跳过 SPA 测试")
	}

	tmp := filepath.Join(os.TempDir(), "tm_spa_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmp,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_spa_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)

	// 根路径返回 index.html
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`<div id="app">`)) {
		t.Fatalf("root should serve index.html: %d", rec.Code)
	}

	// 前端路由（如 /teams/1）回退到 index.html
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/teams/1", nil))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`<div id="app">`)) {
		t.Fatalf("spa fallback failed: %d", rec.Code)
	}

	// 静态资源正常返回
	assets, err := filepath.Glob(filepath.Join("web", "dist", "assets", "index-*.js"))
	if err != nil || len(assets) == 0 {
		t.Fatal("no built js asset found")
	}
	rec = httptest.NewRecorder()
	rel := "/assets/" + filepath.Base(assets[0])
	r.ServeHTTP(rec, httptest.NewRequest("GET", rel, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("asset should be served: %d", rec.Code)
	}
}

func doJSON(t *testing.T, handler http.Handler, path, method string, body []byte, token string) *apiResp {
	return doReq(t, handler, path, method, body, "", token)
}

func doReq(t *testing.T, handler http.Handler, path, method string, body []byte, contentType, token string) *apiResp {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return &apiResp{StatusCode: rec.Code, Header: rec.Header(), Body: rec.Body.Bytes()}
}

func buildMultipart(t *testing.T, field, filename, content string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

func mustOK(t *testing.T, resp *apiResp) {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", resp.StatusCode, string(resp.Body))
	}
	var r struct {
		Code int `json:"code"`
	}
	json.Unmarshal(resp.Body, &r)
	if r.Code != 0 {
		t.Fatalf("unexpected code %d: %s", r.Code, string(resp.Body))
	}
}
