package main

import (
	"log"

	"github.com/ysicing/go-template/internal/bootstrap"
)

// @title go-template API
// @version 0.1.0
// @description go-template 的后端 API，包含安装向导、认证、管理员用户管理与系统设置接口。
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 使用 Bearer access token，例如：Bearer <access_token>
func main() {
	if err := bootstrap.Run(); err != nil {
		log.Fatal(err)
	}
}
