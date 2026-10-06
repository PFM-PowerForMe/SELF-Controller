package controller

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

type runner struct {
	proc         Process
	restartDelay time.Duration
	logger       *log.Logger

	mu  sync.Mutex
	cmd *exec.Cmd
}

func newRunner(proc Process, restartDelay time.Duration, logger *log.Logger) *runner {
	return &runner{proc: proc, restartDelay: restartDelay, logger: logger}
}

func (r *runner) start(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			r.runOnce()
			if ctx.Err() != nil || r.proc.Restart == nil || !*r.proc.Restart {
				return
			}
			r.logger.Printf("%s %s 后重启", r.proc.Name, r.restartDelay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(r.restartDelay):
			}
		}
	}()
}

func (r *runner) runOnce() {
	cmd := exec.Command(r.proc.Command[0], r.proc.Command[1:]...)
	cmd.Dir = r.proc.Dir
	cmd.Env = buildEnv(r.proc.Env)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = credential(r.proc)

	if err := cmd.Start(); err != nil {
		r.logger.Printf("❌ %s 启动失败: %v", r.proc.Name, err)
		return
	}

	r.mu.Lock()
	r.cmd = cmd
	r.mu.Unlock()
	r.logger.Printf("%s 已启动 (pid %d): %s", r.proc.Name, cmd.Process.Pid, strings.Join(r.proc.Command, " "))

	err := cmd.Wait()

	r.mu.Lock()
	r.cmd = nil
	r.mu.Unlock()
	r.logger.Printf("%s 已退出 (%s)", r.proc.Name, exitDesc(cmd, err))
}

func (r *runner) stop(grace time.Duration) {
	r.mu.Lock()
	cmd := r.cmd
	r.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		running := r.cmd != nil
		r.mu.Unlock()
		if !running {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	r.logger.Printf("%s 未在 %s 内退出, 强制结束", r.proc.Name, grace)
	_ = cmd.Process.Kill()
}

func buildEnv(extra map[string]string) []string {
	env := os.Environ()
	for key, value := range extra {
		env = setEnv(env, key, value)
	}
	return env
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func credential(proc Process) *syscall.SysProcAttr {
	if proc.UID == nil && proc.GID == nil {
		return nil
	}
	cred := &syscall.Credential{}
	if proc.UID != nil {
		cred.Uid = uint32(*proc.UID)
	}
	if proc.GID != nil {
		cred.Gid = uint32(*proc.GID)
	}
	return &syscall.SysProcAttr{Credential: cred}
}

func exitDesc(cmd *exec.Cmd, err error) string {
	if cmd.ProcessState != nil {
		if code := cmd.ProcessState.ExitCode(); code >= 0 {
			return fmt.Sprintf("退出码 %d", code)
		}
		if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return fmt.Sprintf("被信号 %s 终止", status.Signal())
		}
	}
	return fmt.Sprintf("异常: %v", err)
}
