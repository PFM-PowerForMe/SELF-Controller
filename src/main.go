package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/pfm-powerforme/self-controller/backup"
	"github.com/pfm-powerforme/self-controller/controller"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return controller.Run(version)
	}
	switch args[0] {
	case "run", "serve":
		return controller.Run(version)
	case "backup":
		if len(args) > 1 && isHelp(args[1]) {
			backup.Help()
			return 0
		}
		if err := backup.Run(); err != nil {
			return 1
		}
		return 0
	case "help", "-h", "--help":
		usage()
		return 0
	case "version", "-v", "--version":
		fmt.Println(version)
		return 0
	default:
		if isHelp(args[0]) {
			usage()
			return 0
		}
		path, err := exec.LookPath(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "找不到命令: %s\n\n", args[0])
			usage()
			return 1
		}
		if err := syscall.Exec(path, args, os.Environ()); err != nil {
			fmt.Fprintf(os.Stderr, "执行 %s 失败: %v\n", path, err)
			return 1
		}
		return 0
	}
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func usage() {
	fmt.Printf(`SELF-Controller %s

用法:
  controller                     按配置文件启动并托管进程, 到点执行备份
  controller run                 同上
  controller backup              立即执行一次备份
  controller backup help         显示备份的环境变量说明
  controller help                显示本帮助
  controller version             显示版本
  controller <命令> [参数...]    直接执行该命令, 不做托管

配置:
  CR_CONTROLLER_CONFIG           配置文件路径, 默认 %s

`, version, controller.DefaultConfigPath)
	backup.Help()
}
