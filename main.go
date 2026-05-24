package main

import (
	"NetworkAuth/cmd"
	"NetworkAuth/server"
	"embed"
	"io/fs"
)

//go:embed all:frontend/dist
var embeddedFrontendDist embed.FS

// main 是程序的入口点
// 调用Cobra命令执行器来处理命令行参数和子命令
func main() {
	distFS, err := fs.Sub(embeddedFrontendDist, "frontend/dist")
	if err != nil {
		panic("Failed to initialize embedded static files: " + err.Error())
	}
	server.SetFrontendFS(distFS)
	cmd.Execute()
}
