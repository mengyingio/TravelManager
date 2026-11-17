/**
 * 版权所有 © 云电工作室（https://www.ce-studio.cn）
 * 本项目由云电工作室成员共同创作编写
 * 以 MIT License 开源，详见项目根目录 LICENSE 文件
 */

package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
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

// buildMultipartForm 构造含表单字段与文件的 multipart 请求体
func buildMultipartForm(fields map[string]string, files map[string]string) ([]byte, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	for field, filename := range files {
		fw, _ := w.CreateFormFile(field, filename)
		_, _ = fw.Write([]byte("fake-content-for-" + filename))
	}
	_ = w.Close()
	return buf.Bytes(), w.FormDataContentType()
}

// TestCollectAndExpenseAttachment 消费收集链接 + 消费附件上传 + 上传限制
func TestCollectAndExpenseAttachment(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "tm_collect_test.db")
	_ = os.Remove(tmp)
	cfg := &config.Config{
		Port:              "0",
		DBPath:            tmp,
		JWTSecret:         "test-secret",
		InitialAdminToken: "test-initial-admin-token",
		UploadDir:         filepath.Join(os.TempDir(), "tm_collect_uploads"),
		TokenTTLHours:     1,
	}
	if err := database.Init(cfg); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r := router.New(cfg)
	admin := registerAdmin(t, r)
	agentID := createAgent(t, r, admin, "收集旅行社")
	body, _ := json.Marshal(map[string]any{"name": "消费收集测试团", "agent_id": agentID, "people_count": 10, "status": "booked"})
	resp := doJSON(t, r, "/api/teams", "POST", body, admin)
	mustOK(t, resp)
	var team struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &team)
	tid := strconv.FormatUint(uint64(team.Data.ID), 10)

	// 开启收集
	body, _ = json.Marshal(map[string]any{"open": true})
	resp = doJSON(t, r, "/api/teams/"+tid+"/collect", "PUT", body, admin)
	mustOK(t, resp)
	var col struct {
		Data struct {
			Open  bool   `json:"open"`
			Token string `json:"token"`
			URL   string `json:"url"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &col)
	if !col.Data.Open || col.Data.Token == "" || !strings.HasPrefix(col.Data.URL, "/collect/") {
		t.Fatalf("collect not enabled: %s", string(resp.Body))
	}
	token := col.Data.Token

	// 公开信息
	resp = doJSON(t, r, "/api/public/collect/"+token, "GET", nil, "")
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"name":"消费收集测试团"`) ||
		!strings.Contains(string(resp.Body), `"max_size_mb":5`) ||
		!strings.Contains(string(resp.Body), `"门票"`) {
		t.Fatalf("collect info unexpected: %s", string(resp.Body))
	}

	// 公开提交（含附件 jpg）
	bodyBytes, ct := buildMultipartForm(
		map[string]string{"category_name": "保险", "description": "意外险", "amount": "300"},
		map[string]string{"file": "receipt.jpg"},
	)
	resp = doReq(t, r, "/api/public/collect/"+token+"/expenses", "POST", bodyBytes, ct, "")
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"category_name":"保险"`) {
		t.Fatalf("collected expense unexpected: %s", string(resp.Body))
	}

	// 团队详情应包含该消费及其附件
	resp = doJSON(t, r, "/api/teams/"+tid, "GET", nil, admin)
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"category_name":"保险"`) ||
		!strings.Contains(string(resp.Body), `"file_name":"receipt.jpg"`) {
		t.Fatalf("expense/attachment not persisted: %s", string(resp.Body))
	}

	// 上传限制：只允许 jpg/pdf，txt 应被拒绝
	body, _ = json.Marshal(map[string]string{"upload_max_size_mb": "5", "upload_allowed_exts": "jpg,pdf"})
	resp = doJSON(t, r, "/api/settings", "PUT", body, admin)
	mustOK(t, resp)

	// 先录入一条消费，再对其实例上传附件
	body, _ = json.Marshal(map[string]any{"category": "meal", "category_name": "餐饮", "description": "午餐", "amount": 200})
	resp = doJSON(t, r, "/api/teams/"+tid+"/expenses", "POST", body, admin)
	mustOK(t, resp)
	var exp struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body, &exp)
	eid := strconv.FormatUint(uint64(exp.Data.ID), 10)

	badBody, badCt := buildMultipart(t, "file", "note.txt", "hello")
	resp = doReq(t, r, "/api/teams/"+tid+"/expenses/"+eid+"/attachments", "POST", badBody, badCt, admin)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(resp.Body), "不支持的文件格式") {
		t.Fatalf("txt upload should be rejected, got %d %s", resp.StatusCode, string(resp.Body))
	}

	okBody, okCt := buildMultipart(t, "file", "bill.jpg", "jpeg-bytes")
	resp = doReq(t, r, "/api/teams/"+tid+"/expenses/"+eid+"/attachments", "POST", okBody, okCt, admin)
	mustOK(t, resp)
	if !strings.Contains(string(resp.Body), `"file_name":"bill.jpg"`) {
		t.Fatalf("jpg upload failed: %s", string(resp.Body))
	}

	// 关闭收集后公开链接失效
	body, _ = json.Marshal(map[string]any{"open": false})
	resp = doJSON(t, r, "/api/teams/"+tid+"/collect", "PUT", body, admin)
	mustOK(t, resp)
	resp = doJSON(t, r, "/api/public/collect/"+token, "GET", nil, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("closed collect link should 404, got %d", resp.StatusCode)
	}

	t.Log("collect / expense attachment test passed")
}
