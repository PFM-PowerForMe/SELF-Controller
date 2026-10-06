package controller

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func Run(version string) int {
	logger := log.New(os.Stdout, "[controller] ", log.LstdFlags|log.Lmsgprefix)
	logger.Printf("SELF-Controller %s 启动 (配置 %s)", version, ConfigPath())

	cfg, err := Load()
	if err != nil {
		logger.Printf("❌ 启动失败: %v", err)
		return 1
	}

	if err := prepareDirs(cfg.Dirs, logger); err != nil {
		logger.Printf("❌ 准备目录失败: %v", err)
		return 1
	}
	if err := renderTemplates(cfg.Templates, logger); err != nil {
		logger.Printf("❌ 生成配置失败: %v", err)
		return 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	runners := make([]*runner, 0, len(cfg.Processes))
	for _, proc := range cfg.Processes {
		r := newRunner(proc, cfg.restartDelay(), logger)
		r.start(ctx, &wg)
		runners = append(runners, r)
	}
	if cfg.Backup != nil {
		wg.Add(1)
		go runBackupJob(ctx, &wg, cfg.Backup, logger, cfg.stopGrace())
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)
	received := <-signals
	logger.Printf("收到信号 %s, 停止服务 (最多等 %s)", received, cfg.stopGrace())

	cancel()
	var stopWG sync.WaitGroup
	for _, r := range runners {
		stopWG.Add(1)
		go func(r *runner) {
			defer stopWG.Done()
			r.stop(cfg.stopGrace())
		}(r)
	}
	stopWG.Wait()
	wg.Wait()

	logger.Printf("已退出")
	return 0
}
