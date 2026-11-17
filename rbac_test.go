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
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"TravelManager/config"
	"TravelManager/database"
	"TravelManager/models"
	"TravelManager/router"
	"TravelManager/services"
)

// TestRBACAndUsers 用户管理 + 角色权限收口 + 设置隔离
func TestRBACAndUsers(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_rbac_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmp,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_rbac_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)

	admin := registerAdmin(t, r)

	// 管理员创建两个旅行社
	agentA := createAgent(t, r, admin, "A旅行社")
	agentB := createAgent(t, r, admin, "B旅行社")

	// 管理员创建旅行社角色用户（绑定 A 社）
	createUser(t, r, admin, map[string]any{
		"username": "agent_a", "password": "pass1234", "name": "A社账号",
		"phone": "13800138001", "email": "agent@a.com", "role": "agent_owner", "agent_id": agentA,
	})
	// 管理员创建负责人角色用户（邮箱匹配团队负责人）
	createUser(t, r, admin, map[string]any{
		"username": "leader1", "password": "pass1234", "name": "带队刘老师",
		"phone": "13900139001", "email": "liu@team.com", "role": "team_leader",
	})

	// 非管理员调用用户管理接口应 403
	agentToken := loginToken(t, r, "agent_a", "pass1234")
	resp := doJSON(t, r, "/api/users", "GET", nil, agentToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("agent_owner should not list users, got %d", resp.StatusCode)
	}
	resp = doJSON(t, r, "/api/settings", "GET", nil, agentToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("agent_owner should not read settings, got %d", resp.StatusCode)
	}
	resp = doJSON(t, r, "/api/logs", "GET", nil, agentToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("agent_owner should not read logs, got %d", resp.StatusCode)
	}
	// 旅行社不可新增/删除旅行社
	body, _ := json.Marshal(map[string]string{"name": "越权旅行社"})
	resp = doJSON(t, r, "/api/agents", "POST", body, agentToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("agent_owner should not create agent, got %d", resp.StatusCode)
	}

	// 旅行社只能看到自己绑定的旅行社
	resp = doJSON(t, r, "/api/agents", "GET", nil, agentToken)
	var agents struct {
		Data []struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &agents)
	if len(agents.Data) != 1 || agents.Data[0].ID != agentA {
		t.Fatalf("agent_owner should only see own agent, got %+v", agents.Data)
	}

	// 旅行社只能为本社创建团队
	ownTeam := createTeam(t, r, agentToken, agentA, "A社研学团", "liu@team.com", "13900139001")
	body, _ = json.Marshal(map[string]any{
		"name": "越权团队", "agent_id": agentB, "leader_email": "liu@team.com",
	})
	resp = doJSON(t, r, "/api/teams", "POST", body, agentToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("agent_owner should not create team for other agent, got %d", resp.StatusCode)
	}

	// 负责人登录后仅能看到自己的团队，且不能建团队/改合同
	leaderToken := loginToken(t, r, "leader1", "pass1234")
	resp = doJSON(t, r, "/api/teams", "GET", nil, leaderToken)
	var teams struct {
		Data []struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &teams)
	if len(teams.Data) != 1 || teams.Data[0].ID != ownTeam {
		t.Fatalf("team_leader should only see own team, got %+v", teams.Data)
	}
	body, _ = json.Marshal(map[string]any{"name": "负责人建团", "agent_id": agentA})
	resp = doJSON(t, r, "/api/teams", "POST", body, leaderToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("team_leader should not create team, got %d", resp.StatusCode)
	}
	// 负责人可以给自己团队补录消费
	body, _ = json.Marshal(map[string]any{"category": "meal", "category_name": "餐饮", "description": "午餐", "amount": 1200})
	resp = doJSON(t, r, fmt.Sprintf("/api/teams/%d/expenses", ownTeam), "POST", body, leaderToken)
	mustOK(t, resp)
	// 负责人访问其他团队应 403
	otherTeam := createTeam(t, r, admin, agentB, "B社独立团", "other@team.com", "")
	resp = doJSON(t, r, fmt.Sprintf("/api/teams/%d", otherTeam), "GET", nil, leaderToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("team_leader should not view other team, got %d", resp.StatusCode)
	}

	// 设置持久化（存储类型与发信配置）
	body, _ = json.Marshal(map[string]string{
		"storage_type": "upyun", "upyun_bucket": "travel-bucket", "upyun_operator": "ops",
		"upyun_password": "secret", "smtp_enabled": "true", "smtp_host": "smtp.example.com",
	})
	resp = doJSON(t, r, "/api/settings", "PUT", body, admin)
	mustOK(t, resp)
	resp = doJSON(t, r, "/api/settings", "GET", nil, admin)
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"storage_type":"upyun"`) ||
		!strings.Contains(string(resp.Body), `"smtp_host":"smtp.example.com"`) {
		t.Fatalf("settings not persisted: %s", string(resp.Body))
	}

	// 测试短信（短信宝模式，mock HTTP 接口验证调用链）
	oldClient := services.HTTPClient
	services.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		rec.WriteHeader(http.StatusOK)
		_, _ = rec.WriteString("0")
		return rec.Result(), nil
	})}
	body, _ = json.Marshal(map[string]string{
		"sms_provider": "smsbao", "sms_access_key": "u123", "sms_access_secret": "p123",
		"sms_sign_name": "团队档案系统", "sms_endpoint": "http://api.smsbao.mock",
	})
	resp = doJSON(t, r, "/api/settings", "PUT", body, admin)
	mustOK(t, resp)
	body, _ = json.Marshal(map[string]string{"phone": "13800138000"})
	resp = doJSON(t, r, "/api/settings/test-sms", "POST", body, admin)
	mustOK(t, resp)
	services.HTTPClient = oldClient

	// 停用用户后无法登录
	createUser(t, r, admin, map[string]any{
		"username": "blocked", "password": "pass1234", "name": "停用账号", "role": "team_leader",
	})
	body, _ = json.Marshal(map[string]any{"role": "team_leader", "status": "disabled"})
	resp = doJSON(t, r, "/api/users/"+userIDByUsername(t, r, admin, "blocked"), "PUT", body, admin)
	mustOK(t, resp)
	body, _ = json.Marshal(map[string]string{"username": "blocked", "password": "pass1234"})
	resp = doJSON(t, r, "/api/auth/login", "POST", body, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("disabled user should not login, got %d", resp.StatusCode)
	}

	t.Log("rbac test passed")
}

func TestNoDataRoleIsolation(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_no_data_role_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port: "0", DBPath: tmp, JWTSecret: "test-secret", InitialAdminToken: "test-initial-admin-token",
		UploadDir: filepath.Join(os.TempDir(), "tm_no_data_role_uploads"), TokenTTLHours: 1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	admin := registerAdmin(t, r)

	guide := models.GuideProfile{
		Name: "演示导游", Phone: "13800138011", Email: "guide@example.com", Status: "active",
	}
	if err := database.DB.Create(&guide).Error; err != nil {
		t.Fatalf("create guide fixture: %v", err)
	}
	leader := models.TeamLeaderProfile{
		Name: "演示负责人", Phone: "13800138012", Email: "leader@example.com", Status: "active",
	}
	if err := database.DB.Create(&leader).Error; err != nil {
		t.Fatalf("create leader fixture: %v", err)
	}
	permissions, _ := json.Marshal([]string{
		"guides.view", "guides.manage", "team_leaders.view", "team_leaders.manage",
		"logs.view", "settings.view",
	})
	if err := database.DB.Create(&models.RoleDefinition{
		Key: "demo", Name: "演示角色", Permissions: string(permissions), DataScope: "none",
	}).Error; err != nil {
		t.Fatalf("create demo role: %v", err)
	}
	createUser(t, r, admin, map[string]any{
		"username": "demo_user", "password": "pass1234", "name": "演示用户", "role": "demo",
	})
	demoToken := loginToken(t, r, "demo_user", "pass1234")

	for _, path := range []string{"/api/guides", "/api/team-leaders"} {
		resp := doJSON(t, r, path, http.MethodGet, nil, demoToken)
		mustOK(t, resp)
		var result struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &result); err != nil {
			t.Fatalf("decode %s response: %v", path, err)
		}
		if len(result.Data) != 0 {
			t.Fatalf("no-data role should see no records from %s: %s", path, string(resp.Body))
		}
	}

	body, _ := json.Marshal(map[string]string{
		"name": "越权导游", "phone": "13800138013", "email": "blocked-guide@example.com", "status": "active",
	})
	resp := doJSON(t, r, "/api/guides", http.MethodPost, body, demoToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("no-data role should not create guide, got %d", resp.StatusCode)
	}
	body, _ = json.Marshal(map[string]string{
		"name": "越权负责人", "phone": "13800138014", "email": "blocked-leader@example.com", "status": "active",
	})
	resp = doJSON(t, r, "/api/team-leaders", http.MethodPost, body, demoToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("no-data role should not create team leader, got %d", resp.StatusCode)
	}
	body, _ = json.Marshal(map[string]string{
		"name": "越权修改导游", "phone": guide.Phone, "email": guide.Email, "status": "active",
	})
	resp = doJSON(t, r, fmt.Sprintf("/api/guides/%d", guide.ID), http.MethodPut, body, demoToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("no-data role should not update guide, got %d", resp.StatusCode)
	}
	body, _ = json.Marshal(map[string]string{
		"name": "越权修改负责人", "phone": leader.Phone, "email": leader.Email, "status": "active",
	})
	resp = doJSON(t, r, fmt.Sprintf("/api/team-leaders/%d", leader.ID), http.MethodPut, body, demoToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("no-data role should not update team leader, got %d", resp.StatusCode)
	}

	resp = doJSON(t, r, "/api/logs", http.MethodGet, nil, demoToken)
	mustOK(t, resp)
	var logs struct {
		Data struct {
			List []models.OperationLog `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &logs); err != nil {
		t.Fatalf("decode logs response: %v", err)
	}
	if len(logs.Data.List) != 0 {
		t.Fatalf("no-data role should not read audit logs: %s", string(resp.Body))
	}
	var auditCount int64
	if err := database.DB.Model(&models.OperationLog{}).Count(&auditCount).Error; err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if auditCount == 0 {
		t.Fatal("test setup should contain audit logs to verify they are hidden")
	}

	resp = doJSON(t, r, "/api/settings", http.MethodGet, nil, demoToken)
	mustOK(t, resp)
	body, _ = json.Marshal(map[string]string{"system_name": "不应保存的名称"})
	resp = doJSON(t, r, "/api/settings", http.MethodPut, body, demoToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("view-only settings role should not save, got %d", resp.StatusCode)
	}
	var changed int64
	if err := database.DB.Model(&models.Setting{}).
		Where("`key` = ? AND value = ?", "system_name", "不应保存的名称").Count(&changed).Error; err != nil {
		t.Fatalf("check settings persistence: %v", err)
	}
	if changed != 0 {
		t.Fatal("forbidden settings update was persisted")
	}
}

// TestOAuthCallback 模拟第三方登录网关，验证第三方登录全链路
func TestOAuthCallback(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_oauth_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmp,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_oauth_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)

	// 模拟第三方登录网关（内存 RoundTripper，不监听端口）
	var gotRedirectURI string
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Query().Get("act") {
		case "login":
			gotRedirectURI = req.URL.Query().Get("redirect_uri")
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "url": "https://oauth.mock/start?type=qq"})
		case "callback":
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "msg": "ok", "type": "qq", "access_token": "t123",
				"social_uid": "u123", "faceimg": "https://avatar.mock/u123.png",
				"nickname": "测试用户", "location": "临沂", "gender": "男",
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	oldClient := services.HTTPClient
	services.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		mockHandler.ServeHTTP(rec, req)
		return rec.Result(), nil
	})}
	defer func() { services.HTTPClient = oldClient }()

	admin := registerAdmin(t, r)
	body, _ := json.Marshal(map[string]string{
		"oauth_enabled": "true", "oauth_appid": "app123", "oauth_appkey": "key456",
		"oauth_base_url": "https://oauth.mock", "oauth_redirect_uri": "http://localhost:8080/api/auth/oauth/callback",
	})
	resp := doJSON(t, r, "/api/settings", "PUT", body, admin)
	mustOK(t, resp)

	// 未开启时应提示（先验证默认，再开启）
	resp = doJSON(t, r, "/api/auth/oauth/authorize?type=qq", "GET", nil, "")
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("authorize before enable should fail")
	}
	_ = resp

	body, _ = json.Marshal(map[string]string{"oauth_enabled": "true"})
	resp = doJSON(t, r, "/api/settings", "PUT", body, admin)
	mustOK(t, resp)

	// 公开系统信息应反映第三方登录开关（登录页据此隐藏/显示入口）
	resp = doJSON(t, r, "/api/system/info", "GET", nil, "")
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"oauth_enabled":true`) {
		t.Fatalf("system info should reflect oauth_enabled: %s", string(resp.Body))
	}

	// 跳转登录页
	resp = doJSON(t, r, "/api/auth/oauth/authorize?type=qq", "GET", nil, "")
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "https://oauth.mock/start?type=qq" {
		t.Fatalf("authorize redirect failed: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	// 回调地址由当前请求域名自动拼接，忽略手动配置值
	if gotRedirectURI != "http://example.com/api/auth/oauth/callback" {
		t.Fatalf("callback should be auto-built from request host, got %s", gotRedirectURI)
	}

	// 未绑定的第三方账号不得自动注册或登录
	resp = doJSON(t, r, "/api/auth/oauth/callback?type=qq&code=once-code", "GET", nil, "")
	if resp.StatusCode != http.StatusForbidden || !strings.Contains(string(resp.Body), "尚未开通") {
		t.Fatalf("unbound oauth account should be rejected, got %d %s", resp.StatusCode, string(resp.Body))
	}
	var adminUser models.User
	if err := database.DB.Where("username = ?", "admin").First(&adminUser).Error; err != nil {
		t.Fatalf("load admin user: %v", err)
	}
	if err := database.DB.Create(&models.UserOAuth{UserID: adminUser.ID, Provider: "qq", SocialUID: "u123"}).Error; err != nil {
		t.Fatalf("create oauth binding: %v", err)
	}

	// 已绑定账号可以登录
	resp = doJSON(t, r, "/api/auth/oauth/callback?type=qq&code=once-code", "GET", nil, "")
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("bound callback should redirect, got %d %s", resp.StatusCode, string(resp.Body))
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/login?code=") {
		t.Fatalf("callback should redirect to login exchange, got %s", loc)
	}
	q := strings.TrimPrefix(loc, "/login?code=")
	if q == "" {
		t.Fatal("empty oauth login code")
	}
	exchangeBody, _ := json.Marshal(map[string]string{"code": q})
	resp = doJSON(t, r, "/api/auth/oauth/exchange", "POST", exchangeBody, "")
	mustOK(t, resp)
	var exchange struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &exchange)
	if exchange.Data.Token == "" {
		t.Fatal("empty oauth token")
	}
	token := exchange.Data.Token
	resp = doJSON(t, r, "/api/auth/me", "GET", nil, token)
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"oauth_uid":"u123"`) {
		t.Fatalf("me should include oauth_uid: %s", string(resp.Body))
	}

	// 再次回调同一 uid 不应重复建号
	resp = doJSON(t, r, "/api/auth/oauth/callback?type=qq&code=once-code-2", "GET", nil, "")
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("second callback failed: %d", resp.StatusCode)
	}
	var count int64
	database.DB.Table("users").Where("oauth_uid = ?", "u123").Count(&count)
	if count != 1 {
		t.Fatalf("oauth user duplicated: %d", count)
	}

	t.Log("oauth test passed")
}

// TestSuperAdmin 首个注册用户自动成为超级管理员；保留角色不可创建 / 降级 / 停用 / 删除
func TestSuperAdmin(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_super_admin_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmp,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_super_admin_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)

	// 初始无用户，注册接口开放；首个注册用户自动成为超级管理员
	var count int64
	if err := database.DB.Model(&models.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatalf("fresh db should have no users, got %d", count)
	}

	admin := registerAdmin(t, r)

	var super models.User
	if err := database.DB.First(&super, 1).Error; err != nil {
		t.Fatalf("no user after register: %v", err)
	}
	if super.Role != "super_admin" {
		t.Fatalf("first registered user should be super_admin, got %s", super.Role)
	}

	// 系统已存在用户，注册关闭，再次注册应被拒绝
	body, _ := json.Marshal(map[string]string{"username": "second", "password": "pass1234"})
	resp := doJSON(t, r, "/api/auth/register", "POST", body, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("register after init should be closed, got %d", resp.StatusCode)
	}

	// 不允许创建超级管理员角色
	body, _ = json.Marshal(map[string]any{
		"username": "hacker", "password": "pass1234", "name": "越权", "role": "super_admin",
	})
	resp = doJSON(t, r, "/api/users", "POST", body, admin)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("creating super_admin should be rejected, got %d", resp.StatusCode)
	}

	// 不允许把普通用户提升为超级管理员
	createUser(t, r, admin, map[string]any{
		"username": "normal", "password": "pass1234", "name": "普通管理员", "role": "admin",
	})
	uid := userIDByUsername(t, r, admin, "normal")
	body, _ = json.Marshal(map[string]any{"role": "super_admin"})
	resp = doJSON(t, r, "/api/users/"+uid, "PUT", body, admin)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("promoting to super_admin should be rejected, got %d", resp.StatusCode)
	}

	// 不允许降级 / 停用 / 删除超级管理员（UID=1）
	body, _ = json.Marshal(map[string]any{"role": "admin"})
	resp = doJSON(t, r, "/api/users/1", "PUT", body, admin)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("demoting super_admin should be rejected, got %d", resp.StatusCode)
	}
	body, _ = json.Marshal(map[string]any{"role": "super_admin", "status": "disabled"})
	resp = doJSON(t, r, "/api/users/1", "PUT", body, admin)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("disabling super_admin should be rejected, got %d", resp.StatusCode)
	}
	resp = doJSON(t, r, "/api/users/1", "DELETE", nil, admin)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("deleting super_admin should be rejected, got %d", resp.StatusCode)
	}

	t.Log("super admin test passed")
}

// ---------- 工具 ----------

func registerAdmin(t *testing.T, r http.Handler) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"username": "admin", "password": "pass1234", "name": "管理员", "setup_token": "test-initial-admin-token",
		"phone": "13800138000", "email": "admin@example.com",
	})
	resp := doJSON(t, r, "/api/auth/register", "POST", body, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register first user failed: %d %s", resp.StatusCode, string(resp.Body))
	}
	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &result)
	if result.Data.Token == "" {
		t.Fatal("empty token after register")
	}
	return result.Data.Token
}
func loginToken(t *testing.T, r http.Handler, username, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp := doJSON(t, r, "/api/auth/login", "POST", body, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login %s failed: %d %s", username, resp.StatusCode, string(resp.Body))
	}
	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &result)
	return result.Data.Token
}

func createAgent(t *testing.T, r http.Handler, token, name string) uint {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"name": name})
	resp := doJSON(t, r, "/api/agents", "POST", body, token)
	mustOK(t, resp)
	var result struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &result)
	return result.Data.ID
}

func createUser(t *testing.T, r http.Handler, token string, user map[string]any) {
	t.Helper()
	body, _ := json.Marshal(user)
	resp := doJSON(t, r, "/api/users", "POST", body, token)
	mustOK(t, resp)
}

func createTeam(t *testing.T, r http.Handler, token string, agentID uint, name, leaderEmail, leaderPhone string) uint {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"name": name, "agent_id": agentID, "leader_email": leaderEmail, "leader_phone": leaderPhone,
	})
	resp := doJSON(t, r, "/api/teams", "POST", body, token)
	mustOK(t, resp)
	var result struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &result)
	return result.Data.ID
}

func userIDByUsername(t *testing.T, r http.Handler, token, username string) string {
	t.Helper()
	resp := doJSON(t, r, "/api/users?keyword="+username, "GET", nil, token)
	mustOK(t, resp)
	var result struct {
		Data []struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &result)
	if len(result.Data) != 1 {
		t.Fatalf("user %s not found: %s", username, string(resp.Body))
	}
	return fmt.Sprintf("%d", result.Data[0].ID)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
