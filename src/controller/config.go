package controller

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const DefaultConfigPath = "/etc/controller/config.json"

type Config struct {
	Dirs         []Dir      `json:"dirs"`
	Templates    []Template `json:"templates"`
	Processes    []Process  `json:"processes"`
	Backup       *BackupJob `json:"backup"`
	StopGrace    string     `json:"stopGrace"`
	RestartDelay string     `json:"restartDelay"`
}

type Dir struct {
	Path      string `json:"path"`
	Owner     string `json:"owner"`
	Mode      string `json:"mode"`
	Recursive bool   `json:"recursive"`
}

type Template struct {
	In       string            `json:"in"`
	Out      string            `json:"out"`
	Mode     string            `json:"mode"`
	Owner    string            `json:"owner"`
	Vars     map[string]string `json:"vars"`
	Commands [][]string        `json:"commands"`
}

type Process struct {
	Name    string            `json:"name"`
	Command []string          `json:"command"`
	Dir     string            `json:"dir"`
	UID     *int              `json:"uid"`
	GID     *int              `json:"gid"`
	Env     map[string]string `json:"env"`
	Restart *bool             `json:"restart"`
}

type BackupJob struct {
	At       string   `json:"at"`
	Timezone string   `json:"timezone"`
	Command  []string `json:"command"`
}

func ConfigPath() string {
	if path := os.Getenv("CR_CONTROLLER_CONFIG"); path != "" {
		return path
	}
	return DefaultConfigPath
}

func Load() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置 %s 失败: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置 %s 失败: %w", path, err)
	}
	if len(cfg.Processes) == 0 {
		return nil, fmt.Errorf("配置 %s 里没有进程 (processes)", path)
	}

	for i := range cfg.Processes {
		p := &cfg.Processes[i]
		if p.Name == "" {
			p.Name = fmt.Sprintf("process-%d", i+1)
		}
		if len(p.Command) == 0 {
			return nil, fmt.Errorf("进程 %s 没有 command", p.Name)
		}
		if p.Restart == nil {
			restart := true
			p.Restart = &restart
		}
	}

	if cfg.Backup != nil {
		if at := os.Getenv("CR_BACKUP_AT"); at != "" {
			cfg.Backup.At = at
		}
		if tz := os.Getenv("CR_BACKUP_TZ"); tz != "" {
			cfg.Backup.Timezone = tz
		}
		if cfg.Backup.At == "" {
			cfg.Backup.At = "21:01"
		}
		if cfg.Backup.Timezone == "" {
			cfg.Backup.Timezone = "Asia/Shanghai"
		}
		if len(cfg.Backup.Command) == 0 {
			return nil, fmt.Errorf("备份任务没有 command")
		}
	}

	return &cfg, nil
}

func (c *Config) stopGrace() time.Duration {
	return parseDuration(c.StopGrace, 5*time.Second)
}

func (c *Config) restartDelay() time.Duration {
	return parseDuration(c.RestartDelay, time.Second)
}

func parseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return def
	}
	return d
}
