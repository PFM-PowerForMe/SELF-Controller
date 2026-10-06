# SELF-Controller

容器 PID1 控制器：按配置文件拉起并托管进程，到点执行备份（打包 + GPG 加密 + 上传 OSS）。

![Alt](screenshot/screenshot.jpg)

## 用法

| 命令 | 说明 |
| :--- | :--- |
| `controller` / `controller run` | 按 `CR_CONTROLLER_CONFIG`（默认 `/etc/controller/config.json`）启动并托管 |
| `controller backup` | 立即执行一次备份 |
| `controller backup help` | 显示备份的环境变量说明 |
| `controller help` / `controller version` | 帮助 / 版本 |
| `controller <命令> [参数...]` | 直接执行该命令（execve），不做托管 |

## 配置文件

```json
{
  "dirs": [
    { "path": "/opt/app", "owner": "65532:65532", "recursive": true }
  ],
  "templates": [
    {
      "in": "/etc/caddy/templates/app.tmpl",
      "out": "/tmp/caddy/Caddyfile",
      "mode": "0644",
      "owner": "65532:65532",
      "vars": { "CR_CADDY_REAL_IP": "X-Forwarded-For" },
      "commands": [["/usr/bin/caddy", "fmt", "--overwrite", "{out}"]]
    }
  ],
  "processes": [
    {
      "name": "app",
      "command": ["/opt/app/server"],
      "dir": "/opt/app",
      "uid": 65532,
      "gid": 65532,
      "env": { "HOME": "/home/nonroot" },
      "restart": true
    }
  ],
  "backup": { "at": "21:01", "timezone": "Asia/Shanghai", "command": ["/usr/bin/controller", "backup"] },
  "stopGrace": "5s",
  "restartDelay": "1s"
}
```

| 字段 | 说明 |
| :--- | :--- |
| `dirs` | 启动前创建并 chown 的目录；`recursive` 为 `true` 时整棵递归 chown |
| `templates` | 模板渲染：`in` 读入、`$VAR` / `${VAR}` / `${VAR:-默认}` 按「环境变量 → `vars` → 默认值」替换、写到 `out`，随后按顺序执行 `commands`（`{in}` / `{out}` 会被替换成对应路径） |
| `processes` | 要托管的进程；`uid` / `gid` 省略时以 root 运行，`restart` 默认 `true`（退出后 `restartDelay` 重启） |
| `backup` | 定时备份：`at` 为 `HH:MM`，`timezone` 默认 `Asia/Shanghai`；到点后以子进程执行 `command` |
| `stopGrace` / `restartDelay` | 收到退出信号后给子进程的宽限时间（默认 `5s`） / 进程退出到重启的间隔（默认 `1s`） |

## 环境变量

| 变量 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `CR_CONTROLLER_CONFIG` | `/etc/controller/config.json` | 配置文件路径 |
| `CR_BACKUP_AT` | `backup.at` | 覆盖备份时刻，格式 `HH:MM` |
| `CR_BACKUP_TZ` | `backup.timezone` | 覆盖备份时区 |
| `CR_AUTOBACKUP_BACKUP_NAME` | `app-backup` | 备份名称前缀 |
| `CR_AUTOBACKUP_BACKUP_PATH` | 无 | 要打包的本地目录，必填 |
| `CR_AUTOBACKUP_ENCRYPTION_PUB_KEY` | 无 | Base64 编码的 GPG 公钥（ASCII Armored），必填 |
| `CR_AUTOBACKUP_OSS_ACCESS_KEY_ID` | 无 | 阿里云 OSS AccessKey ID，必填 |
| `CR_AUTOBACKUP_OSS_ACCESS_KEY_SECRET` | 无 | 阿里云 OSS AccessKey Secret，必填 |
| `CR_AUTOBACKUP_OSS_BUCKET` | 无 | 阿里云 OSS Bucket，必填 |
| `CR_AUTOBACKUP_OSS_REGION` | 无 | 阿里云 OSS Region（如 `cn-hongkong`，自动拼域名），必填 |
| `CR_AUTOBACKUP_PUSH_URL` | 无 | 备份日志推送的 Webhook URL |
| `CR_AUTOBACKUP_PUSH_TOKEN` | 无 | 备份日志推送的 Token |
| `CR_AUTOBACKUP_DAYS_TO_KEEP` | `7` | OSS 上保留旧备份的天数 |

`CR_AUTOBACKUP_ENCRYPTION_PUB_KEY` 是 Base64 编码的 GPG 公钥，避免公钥在环境变量里乱码：

```bash
gpg --armor --output my_pub_key.asc --export "gpg id"
base64 -w 0 my_pub_key.asc
```

解密备份文件：

```bash
gpg --output test.tar.gz --decrypt test.tar.gz.gpg
gzip -t test.tar.gz      # 必须无输出，有报错说明这份备份不能用
tar -xzf test.tar.gz -C /tmp/restore
```

## 备份自检

一轮备份分三步，任一步失败都会报错退出、不会上传：

1. 打包成 `tar.gz`；
2. **回读自检**：完整解压一遍，校验 gzip 完整性（deflate 流损坏、CRC/长度不符都会被抓到），日志打 `归档自检通过`；
3. 加密：归档先落地再加密（不再把 gzip 的小块写入直接喂给加密流），完成后检查密文体积不小于归档，日志打 `已加密`。

看到 `归档自检通过` 与 `已加密` 才代表这一轮产出可用；`gzip -t` 是恢复前最后一道检查。
