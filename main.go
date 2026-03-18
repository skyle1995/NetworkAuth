package main

import "NetworkAuth/cmd"

// main 是程序的入口点
// 调用Cobra命令执行器来处理命令行参数和子命令
func main() {
	cmd.Execute()
}
