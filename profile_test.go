/**
 * 版权所有 © 云电工作室（https://www.ce-studio.cn）
 * 本项目由云电工作室成员共同创作编写
 * 以 MIT License 开源，详见项目根目录 LICENSE 文件
 */

package main

import (
	"encoding/json"
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

// TestMyProfile 个人主页：更新资料 / 上传头像 / 修改密码
func TestMyProfile(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_profile_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmp,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_profile_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	admin := registerAdmin(t, r)

	// 更新个人资料
	body, _ := json.Marshal(map[string]string{
		"name": "超级管理员", "nickname": "老王", "phone": "13900139000", "email": "wang@example.com",
	})
	resp := doJSON(t, r, "/api/users/me", "PUT", body, admin)
	mustOK(t, resp)
	resp = doJSON(t, r, "/api/auth/me", "GET", nil, admin)
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"nickname":"老王"`) ||
		!strings.Contains(string(resp.Body), `"email":"wang@example.com"`) {
		t.Fatalf("profile not updated: %s", string(resp.Body))
	}

	// 上传头像（本地存储 → /uploads/avatars/...）
	avatarBody, ct := buildMultipart(t, "file", "avatar.png", "fake-png-bytes")
	resp = doReq(t, r, "/api/users/me/avatar", "POST", avatarBody, ct, admin)
	mustOK(t, resp)
	var av struct {
		Data struct {
			Avatar string `json:"avatar"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &av)
	if !strings.HasPrefix(av.Data.Avatar, "/uploads/avatars/") {
		t.Fatalf("avatar path unexpected: %s", av.Data.Avatar)
	}
	resp = doJSON(t, r, "/api/auth/me", "GET", nil, admin)
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"avatar":"`+av.Data.Avatar+`"`) {
		t.Fatalf("avatar not saved to profile: %s", string(resp.Body))
	}

	// 修改密码（已有密码需原密码）
	body, _ = json.Marshal(map[string]string{"old_password": "pass1234", "new_password": "newpass123"})
	resp = doJSON(t, r, "/api/users/me/password", "PUT", body, admin)
	mustOK(t, resp)

	t.Log("profile test passed")
}

// mockOAuthGateway 构造内存第三方登录网关
func mockOAuthGateway(t *testing.T) *string {
	t.Helper()
	var gotRedirectURI string
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Query().Get("act") {
		case "login":
			gotRedirectURI = req.URL.Query().Get("redirect_uri")
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "url": "https://oauth.mock/start?type=" + req.URL.Query().Get("type")})
		case "callback":
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "msg": "ok", "type": req.URL.Query().Get("type"), "access_token": "t123",
				"social_uid": "u123", "faceimg": "https://avatar.mock/u123.png",
				"nickname": "测试用户", "location": "临沂", "gender": "男",
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	old := services.HTTPClient
	services.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Result(), nil
	})}
	t.Cleanup(func() { services.HTTPClient = old })
	return &gotRedirectURI
}

func newOAuthTestRouter(t *testing.T, name string) http.Handler {
	t.Helper()
	tmp := filepath.Join(os.TempDir(), "tm_oauth_bind_"+name+".db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmp,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_oauth_bind_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	mockOAuthGateway(t)
	admin := registerAdmin(t, r)
	body, _ := json.Marshal(map[string]string{
		"oauth_enabled": "true", "oauth_appid": "app123", "oauth_appkey": "key456",
		"oauth_base_url": "https://oauth.mock",
	})
	if resp := doJSON(t, r, "/api/settings", "PUT", body, admin); resp.StatusCode != http.StatusOK {
		t.Fatalf("enable oauth failed: %d %s", resp.StatusCode, string(resp.Body))
	}
	return r
}

// TestOAuthBind 个人主页绑定/解绑第三方登录
func TestOAuthBind(t *testing.T) {
	r := newOAuthTestRouter(t, "bind")

	// 首次登录：普通账号
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "pass1234"})
	resp := doJSON(t, r, "/api/auth/login", "POST", body, "")
	mustOK(t, resp)
	var login struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &login)
	token := login.Data.Token

	// 生成绑定票据
	body, _ = json.Marshal(map[string]string{"provider": "qq"})
	resp = doJSON(t, r, "/api/users/me/oauth/ticket", "POST", body, token)
	mustOK(t, resp)
	var tr struct {
		Data struct {
			Ticket string `json:"ticket"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &tr)
	if tr.Data.Ticket == "" {
		t.Fatal("empty bind ticket")
	}

	// 携带票据跳转授权页
	resp = doJSON(t, r, "/api/auth/oauth/authorize?type=qq&bind="+tr.Data.Ticket, "GET", nil, "")
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("authorize bind failed: %d", resp.StatusCode)
	}

	// 回调绑定
	resp = doJSON(t, r, "/api/auth/oauth/callback?type=qq&code=once-code&bind="+tr.Data.Ticket, "GET", nil, "")
	if resp.StatusCode != http.StatusFound || !strings.HasPrefix(resp.Header.Get("Location"), "/profile?oauth=bound&provider=qq") {
		t.Fatalf("bind callback redirect unexpected: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}

	// 用户已绑定 qq
	resp = doJSON(t, r, "/api/auth/me", "GET", nil, token)
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"provider":"qq"`) ||
		!strings.Contains(string(resp.Body), `"social_uid":"u123"`) ||
		!strings.Contains(string(resp.Body), `"oauth_provider":"qq"`) {
		t.Fatalf("bindings not reflected: %s", string(resp.Body))
	}

	// 再次使用同一票据应失效
	resp = doJSON(t, r, "/api/auth/oauth/callback?type=qq&code=once-code-2&bind="+tr.Data.Ticket, "GET", nil, "")
	if resp.StatusCode != http.StatusFound || !strings.Contains(resp.Header.Get("Location"), "oauth=error") {
		t.Fatalf("reused ticket should fail: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}

	// 解绑（该账号有密码，允许）
	resp = doJSON(t, r, "/api/users/me/oauth/qq", "DELETE", nil, token)
	mustOK(t, resp)
	resp = doJSON(t, r, "/api/auth/me", "GET", nil, token)
	mustOK(t, resp)
	if strings.Contains(string(resp.Body), `"provider":"qq"`) {
		t.Fatalf("binding should be removed: %s", string(resp.Body))
	}

	t.Log("oauth bind test passed")
}

// TestOAuthUnbindLockout 无密码的第三方账号解绑保护：先设置密码后才能解绑
func TestOAuthUnbindLockout(t *testing.T) {
	r := newOAuthTestRouter(t, "lockout")

	// 为已创建账号准备一个无密码的第三方绑定，验证解绑保护
	var admin models.User
	if err := database.DB.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}
	database.DB.Model(&admin).Update("has_password", false)
	if err := database.DB.Create(&models.UserOAuth{UserID: admin.ID, Provider: "qq", SocialUID: "u123"}).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}

	// 已绑定账号可以通过第三方回调登录
	resp := doJSON(t, r, "/api/auth/oauth/callback?type=qq&code=once-code", "GET", nil, "")
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("oauth login callback failed: %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	code := strings.TrimPrefix(loc, "/login?code=")
	exchangeBody, _ := json.Marshal(map[string]string{"code": code})
	resp = doJSON(t, r, "/api/auth/oauth/exchange", "POST", exchangeBody, "")
	mustOK(t, resp)
	var exchange struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &exchange)
	token := exchange.Data.Token

	// 无密码且仅剩一个绑定 → 拒绝解绑
	resp = doJSON(t, r, "/api/users/me/oauth/qq", "DELETE", nil, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unbind without password should be rejected, got %d", resp.StatusCode)
	}

	// 设置密码（无需原密码）
	body, _ := json.Marshal(map[string]string{"new_password": "pass1234"})
	resp = doJSON(t, r, "/api/users/me/password", "PUT", body, token)
	mustOK(t, resp)

	// 现在可以解绑
	resp = doJSON(t, r, "/api/users/me/oauth/qq", "DELETE", nil, token)
	mustOK(t, resp)

	t.Log("oauth lockout test passed")
}
