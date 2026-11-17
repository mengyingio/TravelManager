/**
 * 版权所有 © 云电工作室（https://www.ce-studio.cn）
 * 本项目由云电工作室成员共同创作编写
 * 以 MIT License 开源，详见项目根目录 LICENSE 文件
 */

package main

import (
	"log"
	"strings"

	"TravelManager/config"
	"TravelManager/database"
	"TravelManager/router"
	_ "TravelManager/validators"
)

func main() {
	cfg := config.Load()
	if len(strings.TrimSpace(cfg.JWTSecret)) < 32 {
		log.Fatal("JWT_SECRET 未配置或长度不足 32 位，拒绝启动")
	}

	if err := database.Init(cfg); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	r := router.New(cfg)
	log.Printf("TravelManager 服务已启动: http://0.0.0.0:%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
