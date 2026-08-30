# FileHarbor

FileHarbor 是一个面向运维与开发场景的轻量、安全、自托管文件管理器。它以单个 Go 二进制提供受控的文件浏览、上传、下载、编辑、归档及整理能力。

## 功能

- 单管理员登录、12 小时内存会话、CSRF 防护及独立的 API Bearer Token
- 默认只监听 `127.0.0.1`，适合由 Caddy 或 Nginx 终止 HTTPS
- 浏览、下载、安全文本预览、在线编辑、新建文件和目录
- 重命名、移动、复制、只读属性、归档与受控的批量操作
- 多文件可恢复上传队列，真实分片传输进度、暂停、重试、取消与重启后恢复
- 脚本 API 上传，以及旧客户端兼容的直接/分片上传路由（单次上传上限 256 MiB，单片上限 64 MiB）
- 私有运行状态目录，包含审计 JSONL、上传分片和临时 ZIP 文件
- `/healthz` 存活探针、`/readyz` 就绪探针和优雅关停
- `-r` 只读模式，以及允许上传但禁止其它改动的 `-ru` 模式

## 安全配置

FileHarbor 默认强制登录。它仅接受 cost 10–12 的 bcrypt 密码 hash，不接受明文管理员密码。会话密钥和 API Token 必须至少 32 个字符。

| 环境变量 | 可选命令行参数 | 说明 |
| --- | --- | --- |
| `FILEHARBOR_ADMIN_USERNAME` | `-admin-username` | 唯一管理员账号 |
| `FILEHARBOR_ADMIN_PASSWORD_HASH` | `-admin-password-hash` | 管理员密码 bcrypt hash |
| `FILEHARBOR_SESSION_SECRET` | `-session-secret` | 会话签名密钥，至少 32 个字符 |
| `FILEHARBOR_API_TOKEN` | `-api-token` | API 上传 Token，至少 32 个字符 |
| `FILEHARBOR_STATE_DIR` | `-state-dir` | 私有运行状态目录 |

`FILEHARBOR_*` 是新部署唯一推荐的命名空间。为从 goFile 升级，FileHarbor 在过渡期仍接受一整组旧的 `GOFILE_*` 值，但**禁止混用**两个命名空间。`GOFILE_ADMIN_PASSWORD` 和 `FILEHARBOR_ADMIN_PASSWORD` 均会被拒绝。若旧密码曾以明文出现在服务定义、Shell 历史或日志中，请立即轮换密码和所有会话/API 密钥。

使用真实终端生成 hash，不会创建或修改配置文件：

```sh
fileharbor hash-password
```

PowerShell 示例：

```powershell
$hash = & .\fileharbor.exe hash-password
$env:FILEHARBOR_ADMIN_USERNAME = 'admin'
$env:FILEHARBOR_ADMIN_PASSWORD_HASH = $hash
$env:FILEHARBOR_SESSION_SECRET = 'replace-with-a-random-secret-at-least-32-characters'
$env:FILEHARBOR_API_TOKEN = 'replace-with-a-separate-random-token-at-least-32-characters'
.\fileharbor.exe -path 'D:\data' -state-dir 'D:\fileharbor-state'
```

Linux/macOS 示例：

```sh
export FILEHARBOR_ADMIN_USERNAME='admin'
export FILEHARBOR_ADMIN_PASSWORD_HASH="$(fileharbor hash-password)"
export FILEHARBOR_SESSION_SECRET='replace-with-a-random-secret-at-least-32-characters'
export FILEHARBOR_API_TOKEN='replace-with-a-separate-random-token-at-least-32-characters'
fileharbor -path /srv/fileharbor/data -state-dir /var/lib/fileharbor
```

不要把 bcrypt hash、会话密钥或 API Token 放在 URL、Git 仓库或长期保存的命令行中。命令行参数可能出现在历史记录、进程列表和服务定义里，生产环境应使用 root 专用的环境文件或外部密钥管理器。

## 参数与地址

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-path` | `./` | 受管文件根目录，必须已存在 |
| `-state-dir` | 用户缓存目录下的 `fileharbor` | 私有运行状态目录，不能位于或包含受管根目录 |
| `-base-path` | 空 | 公开 URL 子路径，例如 `/fileharbor` |
| `-port` | `8089` | 服务端口 |
| `-host` | `127.0.0.1` | 监听地址 |
| `-r` | `false` | 只读模式 |
| `-ru` | `false` | 只读加上传模式 |
| `-cookie-secure` | `false` | HTTPS 反代时必须设置的 Secure Cookie |
| `-allow-insecure-lan` | `false` | 显式允许非 loopback HTTP，生产环境不安全 |
| `-upload-max-bytes` | `8 GiB` | 单个可靠上传的最大文件大小 |
| `-upload-chunk-bytes` | `8 MiB` | 可靠上传的单片大小，范围为 1–64 MiB |
| `-upload-max-parts` | `4096` | 单个可靠上传允许的最大分片数 |
| `-upload-max-active` | `64` | 同时保留的活动可靠上传数 |
| `-upload-max-pending-bytes` | `16 GiB` | 所有活动可靠上传预留的总大小 |
| `-upload-max-concurrent-parts` | `8` | 同时落盘的可靠上传分片数 |
| `-upload-inactivity-ttl` | `24h` | 活动可靠上传无进展后的回收期限 |
| `-upload-completion-ttl` | `1h` | 完成或取消上传状态的保留期限 |
| `-upload-min-free-bytes` | `256 MiB` | 上传卷和最终目标卷需保留的最小可用空间 |

公开地址为 `https://files.example.com/fileharbor/` 时使用 `-base-path /fileharbor`。反向代理可以保留该前缀，也可以在转发前剥离它，FileHarbor 支持两种方式且不会信任 `X-Forwarded-Prefix`。已有部署可继续使用 `-base-path /gofile`，无需迁移 URL。

使用 HTTP 探针：`/healthz` 表示进程可服务，`/readyz` 仅在私有状态和审计日志可用时返回 200。容器和负载均衡器应使用 `/readyz`。单次 multipart 上传限制为 256 MiB，旧分片上传的每个分片限制为 64 MiB；超过限制会返回 `413`。收到终止信号后，FileHarbor 停止接收新请求并在平台关停时限内等待已接收的请求；若超时，进程会退出并保留状态目录锁，由操作系统在进程退出时释放，避免仍在执行的请求与状态清理发生竞争。

## 可靠可恢复上传 API

内置网页使用 v1 `api/uploads` 协议：可同时排队多个文件，基于真实分片传输显示进度，并提供暂停、恢复、重试与取消。网页会将 upload ID、capability、目标、文件元数据和完整 SHA-256 保存到当前浏览器的 IndexedDB，**从不保存文件内容、`File`、`Blob`、base64 或分片字节**。页面刷新、浏览器重启或重新登录后，浏览器必须重新选择原文件，并完整计算 SHA-256 后才能继续上传；选择的文件不匹配时，不会向已有传输写入任何内容。

`-ru` 模式也可使用该网页队列；严格 `-r` 模式不会显示或注册上传能力。旧的 `/do/upload/*path`、`/api/upload` 和 `/do/chunk/*` 保持兼容，但它们属于尽力而为的直接上传，服务重启后不能恢复，且内置网页不再使用它们。

可靠上传把私有 manifest 与分片保存在 `-state-dir/uploads/`。该目录必须置于独立、持久的卷上，并为每个活动上传预留完整文件大小；完成组装时还需要在目标目录为第二份临时文件预留同等空间。任何以 `.fileharbor-upload-` 开头的名称均为内部保留名称，不能作为普通文件名。

客户端先生成一个不透明上传 ID 和一个随机的 32-byte capability（64 个小写十六进制字符）。capability 仅以 SHA-256 hash 形式保存在私有 manifest 中，绝不能放在 URL、日志、审计记录或 Git 中。每个请求仍需正常认证：浏览器 mutation 需携带当前会话的 `X-CSRF-Token`，Bearer token 仅可访问 `/api/upload` 与 `/api/uploads` 系列路由。

| 方法 | 路径 | 作用 |
| --- | --- | --- |
| `POST` | `/api/uploads` | 创建或幂等恢复传输。JSON: `path`、`name`、`size`、可选 `sha256`；头: `X-Upload-ID`、`X-Upload-Token` |
| `GET` | `/api/uploads/:id` | 读取传输状态与已接收的紧凑分片范围；头: `X-Upload-Token` |
| `PUT` | `/api/uploads/:id/parts/:index` | 上传原始分片；头: `X-Upload-Token`、`X-Upload-Part-SHA256` |
| `POST` | `/api/uploads/:id/complete` | 校验、同目录组装并以不覆盖语义原子发布 |
| `DELETE` | `/api/uploads/:id` | 取消未完成上传，不会删除已发布文件 |

每个分片必须恰好等于协议所需长度且 SHA-256 匹配。相同 checksum 的重试成功且不重复写入，不同内容的重复分片返回 `409 part_conflict`。完成操作会重新校验每个私有分片和全文件 checksum，最终文件名在成功发布前不可见；若目标已被其他操作创建，返回 `409 destination_exists`，不会覆盖它。

响应为 JSON。常见稳定错误码为 `invalid_upload`、`invalid_part`、`invalid_digest`（400），`upload_not_found`（404，亦用于隐藏 owner/capability 不匹配），`upload_expired` / `upload_cancelled`（410），`size_mismatch` / `upload_too_large`（413），`upload_busy`（429），`part_conflict` / `upload_incomplete` / `destination_exists`（409），以及 `insufficient_storage`（507）。客户端在网络中断、进程重启或完成响应丢失后应先 `GET` 状态，再只重传缺失分片或读取已完成结果。

启动恢复会验证持久化 manifest 与每个已记录分片的长度和 checksum。若发布已完成但 completed 状态未写入，服务会只在最终文件与私有分片完整匹配时补写完成状态；不明确或损坏的状态会拒绝启动，而不是猜测、覆盖或静默删除数据。活动传输在 `-upload-inactivity-ttl` 后回收；完成及取消状态在 `-upload-completion-ttl` 后回收。

## systemd 新部署

以下示例让 FileHarbor 仅监听本机，并由 Caddy/Nginx 提供 HTTPS。

```sh
install -o root -g root -m 0755 ./fileharbor /usr/local/bin/fileharbor
useradd --system --home /var/lib/fileharbor --shell /usr/sbin/nologin fileharbor
install -d -o fileharbor -g fileharbor -m 0750 /srv/fileharbor/data /var/lib/fileharbor
install -d -o root -g root -m 0700 /etc/fileharbor
install -o root -g root -m 0600 /dev/null /etc/fileharbor/fileharbor.env
sudo -u fileharbor /usr/local/bin/fileharbor hash-password
openssl rand -hex 32
openssl rand -hex 32
```

在 `/etc/fileharbor/fileharbor.env` 写入四项。bcrypt hash 中的 `$` 必须保留原样：

```ini
FILEHARBOR_ADMIN_USERNAME=admin
FILEHARBOR_ADMIN_PASSWORD_HASH=$2a$10$replace-with-the-complete-bcrypt-hash
FILEHARBOR_SESSION_SECRET=replace-with-the-first-openssl-output
FILEHARBOR_API_TOKEN=replace-with-the-second-openssl-output
```

创建 `/etc/systemd/system/fileharbor.service`：

```ini
[Unit]
Description=FileHarbor Web File Manager
After=network.target

[Service]
Type=simple
User=fileharbor
Group=fileharbor
WorkingDirectory=/var/lib/fileharbor
EnvironmentFile=/etc/fileharbor/fileharbor.env
ExecStart=/usr/local/bin/fileharbor -path /srv/fileharbor/data -state-dir /var/lib/fileharbor -host 127.0.0.1 -port 8089 -cookie-secure -base-path /fileharbor
Restart=on-failure
RestartSec=3
UMask=0077
LimitNOFILE=65535
MemoryMax=200M
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes
LockPersonality=yes
ReadWritePaths=/srv/fileharbor/data /var/lib/fileharbor

[Install]
WantedBy=multi-user.target
```

```sh
systemd-analyze verify /etc/systemd/system/fileharbor.service
systemctl daemon-reload
systemctl enable --now fileharbor
systemctl status fileharbor --no-pager
journalctl -u fileharbor -f
```

不要把秘密写进 `ExecStart=`。如服务启动失败，优先查看 `journalctl -u fileharbor -n 100 --no-pager`。

### 现有 goFile systemd 升级

1. 先备份受管目录和 `/etc/gofile/gofile.env`，停止服务：`systemctl stop gofile`。
2. 首次升级保留原来的 `-path`、用户、数据位置及 `-base-path /gofile`。不要移动数据目录。
3. 新建 `FILEHARBOR_*` 环境文件，或在过渡期只保留完整的旧 `GOFILE_*` 组，不能混合。
4. FileHarbor 的 `-state-dir` 必须在受管目录外，例如 `/var/lib/fileharbor`。不要删除旧的临时遗留文件，先人工检查。
5. 验证 `fileharbor -h` 和 `/readyz` 后，才切换 systemd 单元。旧 `gofile.service` 和帐户可在稳定运行后按自己的变更流程清理。

## Docker Compose

仓库中的 [compose.yaml](compose.yaml) 本地构建 `fileharbor:local`。容器以非 root 的 `fileharbor` 用户运行，根文件系统只读，受管数据保存于 `fileharbor-data`，运行状态保存于独立的 `fileharbor-state` 卷，端口仅发布在 `127.0.0.1:8089`。Compose 配置 `stop_grace_period: 20s`，长于服务在 Unix 上的 15 秒请求 drain 期限；不要在生产编排中设置更短的优雅关停时间。

```sh
mkdir -p /opt/fileharbor
cd /opt/fileharbor
# 放入 FileHarbor 源码或发布源码包
cp fileharbor.env.example fileharbor.env
chown root:root fileharbor.env
chmod 0600 fileharbor.env
nano fileharbor.env
docker compose build
docker compose up -d
docker compose ps
docker compose logs -f fileharbor
```

`fileharbor.env`：

```ini
FILEHARBOR_ADMIN_USERNAME=admin
FILEHARBOR_ADMIN_PASSWORD_HASH='$2a$10$replace-with-the-complete-bcrypt-hash'
FILEHARBOR_SESSION_SECRET=replace-with-a-random-secret-at-least-32-characters
FILEHARBOR_API_TOKEN=replace-with-a-separate-random-token-at-least-32-characters
```

Compose 的 `env_file` 会移除单引号并按字面量传递 bcrypt hash。不要将真实凭据写入 `compose.yaml`，Docker daemon/API 权限等同于读取这些凭据的权限。

容器默认等效启动参数为：

```text
-host 0.0.0.0 -port 8089 -path /data -state-dir /state -allow-insecure-lan
```

端口只发布到宿主机 loopback，因此可直接用 `http://127.0.0.1:8089/` 验证和接入本机反代。该默认值仅适用于这种 loopback HTTP 方式，**不要**将端口改为对外网卡发布。生产 HTTPS 反代必须完整覆盖 Compose 的 `command:`，使用 `-cookie-secure` 并移除 `-allow-insecure-lan`：

```yaml
    command:
      - -host
      - 0.0.0.0
      - -port
      - "8089"
      - -path
      - /data
      - -state-dir
      - /state
      - -cookie-secure
      - -base-path
      - /fileharbor
```

### 从 goFile Compose 迁移

旧部署通常使用 `gofile-data` 卷。**绝不要执行 `docker compose down -v`**，它会删除命名卷。升级时先停止旧服务，保留数据卷，然后将旧卷显式挂载为新服务的 `/data`：

```yaml
services:
  fileharbor:
    volumes:
      - gofile-data:/data
      - fileharbor-state:/state

volumes:
  gofile-data:
    external: true
  fileharbor-state:
```

确认所有数据与 `/readyz` 正常后，再决定是否通过离线复制迁移到新 `fileharbor-data` 卷。不要仅替换 Compose 文件后就假定旧卷会自动改名或迁移。

## API 上传

Bearer Token 只允许上传，不允许浏览、下载或其它管理操作。子路径部署时保留配置的 base path：

```sh
curl -H "Authorization: Bearer $FILEHARBOR_API_TOKEN" \
  -F 'path=logs' \
  -F 'file=@/path/to/app.log' \
  https://files.example.com/fileharbor/api/upload
```

## 安装脚本与发布包

发布包名为 `fileharbor-<os>-<arch>`，其中 Windows 二进制为 `fileharbor.exe`，其他平台为 `fileharbor`。Linux/macOS 可用经过 SHA-256 校验的安装脚本：

```sh
curl -fsSLO https://github.com/irains/fileharbor/releases/latest/download/fileharbor.sh
sudo bash fileharbor.sh
```

在下载或执行任何安装脚本前，请核实发布仓库、版本标签及校验和。

## 开发

Go 没有 LTSC/LTS 发布通道。本项目目标为 Go 1.27.x，并应保持在该补丁线最新版本。

```sh
gofmt -w $(git ls-files '*.go')
go mod tidy
go vet ./...
go test ./...
go test -race ./...
go build ./...
```

发布前应在 Linux 与 Windows 运行完整测试和构建矩阵。Windows 的私有状态目录仍依赖部署帐户的 ACL，Unix 风格的 `0700`/`0600` 模式位不能单独证明 Windows ACL 私密性。
