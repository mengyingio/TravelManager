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
	"strings"
	"testing"

	"TravelManager/config"
	"TravelManager/database"
	"TravelManager/handlers"
	"TravelManager/models"
	"TravelManager/router"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// TestForgotResetPassword 找回密码：发送验证码 → 校验并重置密码
func TestForgotResetPassword(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_forgot_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmp,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_forgot_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	registerAdmin(t, r)

	var gotCode string
	oldSend := handlers.SendPasswordResetCode
	handlers.SendPasswordResetCode = func(user models.User, code string) error {
		gotCode = code
		return nil
	}
	defer func() { handlers.SendPasswordResetCode = oldSend }()

	// 申请验证码（按用户名）
	body, _ := json.Marshal(map[string]string{"account": "admin"})
	resp := doJSON(t, r, "/api/auth/forgot-password", "POST", body, "")
	mustOK(t, resp)
	if gotCode == "" || len(gotCode) != 6 {
		t.Fatalf("reset code not generated: %q", gotCode)
	}

	// 错误验证码应拒绝
	body, _ = json.Marshal(map[string]string{"account": "admin", "code": "000000", "new_password": "newpass123"})
	resp = doJSON(t, r, "/api/auth/reset-password", "POST", body, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong code should fail, got %d", resp.StatusCode)
	}

	// 正确验证码重置成功
	body, _ = json.Marshal(map[string]string{"account": "admin", "code": gotCode, "new_password": "newpass123"})
	resp = doJSON(t, r, "/api/auth/reset-password", "POST", body, "")
	mustOK(t, resp)

	// 验证码一次性，重复使用应失败
	resp = doJSON(t, r, "/api/auth/reset-password", "POST", body, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("reused code should fail, got %d", resp.StatusCode)
	}

	// 新密码可登录，旧密码不可
	body, _ = json.Marshal(map[string]string{"username": "admin", "password": "newpass123"})
	resp = doJSON(t, r, "/api/auth/login", "POST", body, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login with new password failed: %d %s", resp.StatusCode, string(resp.Body))
	}
	body, _ = json.Marshal(map[string]string{"username": "admin", "password": "pass1234"})
	resp = doJSON(t, r, "/api/auth/login", "POST", body, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old password should fail, got %d", resp.StatusCode)
	}

	t.Log("forgot/reset password test passed")
}

// TestPasskey 通行密钥：注册会话 / 登录会话 / 列表 / 删除
func TestPasskey(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_passkey_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmp,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_passkey_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	admin := registerAdmin(t, r)

	// 空列表
	resp := doJSON(t, r, "/api/passkeys", "GET", nil, admin)
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"data":[]`) {
		t.Fatalf("passkey list should be empty: %s", string(resp.Body))
	}

	// 开始注册
	resp = doJSON(t, r, "/api/passkeys/begin-register", "POST", nil, admin)
	mustOK(t, resp)
	var br struct {
		Data struct {
			Session string `json:"session"`
			Options struct {
				PublicKey struct {
					Challenge string `json:"challenge"`
					RP        struct {
						ID string `json:"id"`
					} `json:"rp"`
					User struct {
						ID string `json:"id"`
					} `json:"user"`
				} `json:"publicKey"`
			} `json:"options"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &br)
	if br.Data.Session == "" || br.Data.Options.PublicKey.Challenge == "" ||
		br.Data.Options.PublicKey.RP.ID == "" || br.Data.Options.PublicKey.User.ID == "" {
		t.Fatalf("begin-register options incomplete: %s", string(resp.Body))
	}

	// 无效会话完成注册 → 400
	body, _ := json.Marshal(map[string]any{"session": "bad", "response": map[string]any{"id": "x"}})
	resp = doJSON(t, r, "/api/passkeys/complete-register", "POST", body, admin)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("complete with bad session should fail, got %d", resp.StatusCode)
	}

	// 有效会话但伪造响应 → 400（WebAuthn 校验失败）
	body, _ = json.Marshal(map[string]any{"session": br.Data.Session, "response": map[string]any{"id": "fake", "type": "public-key", "rawId": "ZmFrZQ", "response": map[string]any{}}})
	resp = doJSON(t, r, "/api/passkeys/complete-register", "POST", body, admin)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("fake attestation should fail, got %d %s", resp.StatusCode, string(resp.Body))
	}

	// 开始通行密钥登录（公开）
	resp = doJSON(t, r, "/api/auth/passkey/begin-login", "GET", nil, "")
	mustOK(t, resp)
	var bl struct {
		Data struct {
			Session string `json:"session"`
			Options struct {
				PublicKey struct {
					Challenge string `json:"challenge"`
				} `json:"publicKey"`
			} `json:"options"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &bl)
	if bl.Data.Session == "" || bl.Data.Options.PublicKey.Challenge == "" {
		t.Fatalf("begin-login options incomplete: %s", string(resp.Body))
	}

	// 伪造断言登录 → 400
	body, _ = json.Marshal(map[string]any{"session": bl.Data.Session, "response": map[string]any{"id": "fake", "type": "public-key", "rawId": "ZmFrZQ", "response": map[string]any{}}})
	resp = doJSON(t, r, "/api/auth/passkey/complete-login", "POST", body, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("fake assertion should fail, got %d %s", resp.StatusCode, string(resp.Body))
	}

	// 删除不存在的通行密钥 → 404
	resp = doJSON(t, r, "/api/passkeys/999", "DELETE", nil, admin)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("delete missing passkey should 404, got %d", resp.StatusCode)
	}

	t.Log("passkey test passed")
}

func TestPasskeyCredentialJSONRoundTrip(t *testing.T) {
	cred := webauthn.Credential{
		ID:              []byte{1, 2, 3},
		PublicKey:       []byte{4, 5, 6},
		AttestationType: "none",
		Transport:       []protocol.AuthenticatorTransport{protocol.USB, protocol.Internal},
		Flags:           webauthn.CredentialFlags{UserPresent: true, UserVerified: true},
		Authenticator: webauthn.Authenticator{
			AAGUID:    []byte{9, 9, 9},
			SignCount: 7,
		},
		Attestation: webauthn.CredentialAttestation{
			ClientDataJSON:     []byte("{}"),
			AuthenticatorData:  []byte{1},
			PublicKeyAlgorithm: -7,
			Object:             []byte{2},
		},
	}
	data, err := json.Marshal(cred)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back webauthn.Credential
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(back.ID) != string(cred.ID) || back.Authenticator.SignCount != 7 || len(back.Transport) != 2 {
		t.Fatalf("round trip mismatch: %s", string(data))
	}
	t.Logf("credential json ok: %s", string(data))
}
