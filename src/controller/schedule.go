package controller

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func parseClock(at string) (int, int, error) {
	parts := strings.Split(at, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("时间格式应为 HH:MM, 实际为 %q", at)
	}
	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("小时不是数字: %q", parts[0])
	}
	minute, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("分钟不是数字: %q", parts[1])
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("时间超出范围: %q", at)
	}
	return hour, minute, nil
}

func nextRun(now time.Time, at string, loc *time.Location) (time.Time, error) {
	hour, minute, err := parseClock(at)
	if err != nil {
		return time.Time{}, err
	}
	local := now.In(loc)
	next := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
	if !next.After(local) {
		next = next.AddDate(0, 0, 1)
	}
	return next, nil
}

func runBackupJob(ctx context.Context, wg *sync.WaitGroup, job *BackupJob, logger *log.Logger, grace time.Duration) {
	defer wg.Done()

	loc, err := time.LoadLocation(job.Timezone)
	if err != nil {
		logger.Printf("时区 %s 不可用, 改用 UTC: %v", job.Timezone, err)
		loc = time.UTC
	}

	for {
		next, err := nextRun(time.Now(), job.At, loc)
		if err != nil {
			logger.Printf("❌ 备份时间 %q 不可用: %v", job.At, err)
			return
		}
		logger.Printf("下次备份: %s (%s)", next.Format("2006-01-02 15:04:05"), job.Timezone)

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		runBackupOnce(ctx, job, logger, grace)
	}
}

func runBackupOnce(ctx context.Context, job *BackupJob, logger *log.Logger, grace time.Duration) {
	logger.Printf("开始执行备份: %s", strings.Join(job.Command, " "))

	cmd := exec.Command(job.Command[0], job.Command[1:]...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		logger.Printf("❌ 备份启动失败: %v", err)
		return
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			logger.Printf("❌ 备份失败 (%s)", exitDesc(cmd, err))
			return
		}
		logger.Printf("✅ 备份完成")
	case <-ctx.Done():
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(grace):
			_ = cmd.Process.Kill()
		}
	}
}
