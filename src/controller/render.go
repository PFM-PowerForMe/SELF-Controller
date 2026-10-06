package controller

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var varRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

func substitute(text string, defaults map[string]string) string {
	return varRe.ReplaceAllStringFunc(text, func(match string) string {
		groups := varRe.FindStringSubmatch(match)
		name := groups[1]
		def := groups[3]
		if name == "" {
			name = groups[4]
			def = ""
		}
		if value, ok := os.LookupEnv(name); ok {
			return value
		}
		if defaults != nil {
			if value, ok := defaults[name]; ok {
				return value
			}
		}
		return def
	})
}

func prepareDirs(dirs []Dir, logger *log.Logger) error {
	for _, d := range dirs {
		if d.Path == "" {
			continue
		}
		if err := os.MkdirAll(d.Path, fileMode(d.Mode, 0o755)); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", d.Path, err)
		}
		if d.Mode != "" {
			if err := os.Chmod(d.Path, fileMode(d.Mode, 0o755)); err != nil {
				return fmt.Errorf("设置目录 %s 权限失败: %w", d.Path, err)
			}
		}
		if err := chownPath(d.Path, d.Owner, d.Recursive); err != nil {
			return fmt.Errorf("设置目录 %s 属主失败: %w", d.Path, err)
		}
		logger.Printf("目录就绪: %s (owner=%s recursive=%v)", d.Path, d.Owner, d.Recursive)
	}
	return nil
}

func renderTemplates(templates []Template, logger *log.Logger) error {
	for _, t := range templates {
		if t.In == "" || t.Out == "" {
			return fmt.Errorf("模板配置缺少 in / out")
		}

		data, err := os.ReadFile(t.In)
		if err != nil {
			return fmt.Errorf("读取模板 %s 失败: %w", t.In, err)
		}
		if err := os.MkdirAll(filepath.Dir(t.Out), 0o755); err != nil {
			return fmt.Errorf("创建 %s 的目录失败: %w", t.Out, err)
		}
		mode := fileMode(t.Mode, 0o644)
		if err := os.WriteFile(t.Out, []byte(substitute(string(data), t.Vars)), mode); err != nil {
			return fmt.Errorf("写入 %s 失败: %w", t.Out, err)
		}
		if err := os.Chmod(t.Out, mode); err != nil {
			return fmt.Errorf("设置 %s 权限失败: %w", t.Out, err)
		}

		for _, command := range t.Commands {
			if err := runTemplateCommand(command, t); err != nil {
				return fmt.Errorf("执行 %s 失败: %w", strings.Join(command, " "), err)
			}
		}

		if err := chownPath(t.Out, t.Owner, false); err != nil {
			return fmt.Errorf("设置 %s 属主失败: %w", t.Out, err)
		}
		logger.Printf("已生成 %s (来自 %s)", t.Out, t.In)
	}
	return nil
}

func runTemplateCommand(command []string, t Template) error {
	if len(command) == 0 {
		return nil
	}
	argv := make([]string, len(command))
	for i, arg := range command {
		arg = strings.ReplaceAll(arg, "{in}", t.In)
		argv[i] = strings.ReplaceAll(arg, "{out}", t.Out)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func fileMode(s string, def os.FileMode) os.FileMode {
	if s == "" {
		return def
	}
	value, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return def
	}
	return os.FileMode(value)
}

func parseOwner(s string) (int, int, error) {
	parts := strings.SplitN(s, ":", 2)
	uid, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("owner %q 的 uid 不是数字", s)
	}
	gid := uid
	if len(parts) == 2 {
		gid, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("owner %q 的 gid 不是数字", s)
		}
	}
	return uid, gid, nil
}

func chownPath(path, owner string, recursive bool) error {
	if owner == "" {
		return nil
	}
	uid, gid, err := parseOwner(owner)
	if err != nil {
		return err
	}
	if !recursive {
		return os.Lchown(path, uid, gid)
	}
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(p, uid, gid)
	})
}
