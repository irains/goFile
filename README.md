# goFile

一个面向运维与开发场景的轻量、安全、自托管文件管理器。它以单个 Go 二进制提供浏览、上传、下载、编辑、归档及受控的文件整理能力。

## 功能

- 账号登录、12 小时会话、CSRF 防护和单独的 API Bearer Token
- 默认仅监听 `127.0.0.1`，可通过参数显式指定监听地址
- 浏览、下载、预览、在线编辑、新建文件/文件夹
- 文件和目录的重命名、移动、复制、只读属性查看
- ZIP、TAR.GZ 解压，单目录创建同级 ZIP
- 当前目录多选，批量移动、复制、删除及一次性 ZIP 下载
- 单文件拖放上传和兼容脚本的 API 上传
- 中文/English 自动切换，Bootstrap 5 响应式深色工作台
- `-r` 只读模式，`-ru` 只读加上传模式

## 启动前的安全配置

goFile 默认强制登录。管理员密码只接受 **bcrypt hash**，不接受明文密码。可用环境变量或命令行参数提供配置，未提供的命令行参数会逐字段回退到对应环境变量，显式提供的参数只覆盖自身字段。管理员账号不能为空；密码 hash 必须是 cost 为 10–12 的有效 bcrypt hash；会话签名密钥及 API Token 至少 32 个字符。

| 环境变量 | 可选命令行参数 | 说明 |
| --- | --- | --- |
| `GOFILE_ADMIN_USERNAME` | `-admin-username` | 唯一管理员账号 |
| `GOFILE_ADMIN_PASSWORD_HASH` | `-admin-password-hash` | 管理员密码的 bcrypt hash |
| `GOFILE_SESSION_SECRET` | `-session-secret` | 会话签名密钥，至少 32 个字符，建议使用随机 32 字节以上值 |
| `GOFILE_API_TOKEN` | `-api-token` | 脚本上传 Token，至少 32 个字符，建议使用随机 32 字节以上值 |

`GOFILE_ADMIN_PASSWORD` 和 `-admin-password` 已不再支持。若已配置有效的 bcrypt hash，残留的 `GOFILE_ADMIN_PASSWORD` 不会阻止短暂迁移，但它绝不会作为密码来源，仍应立即删除。若旧密码曾以命令行、CI 日志、服务定义或其他明文方式出现，请在迁移前后轮换该密码，而不仅是将原值生成 hash。

### 生成管理员密码 hash

使用交互式子命令生成 bcrypt hash。它要求真实终端、两次无回显输入密码，且只将 hash 输出到 stdout，不会创建或修改任何配置文件：

```powershell
$hash = & .\goFile.exe hash-password
$env:GOFILE_ADMIN_PASSWORD_HASH = $hash
Remove-Item Env:\GOFILE_ADMIN_PASSWORD -ErrorAction SilentlyContinue
```

```sh
export GOFILE_ADMIN_PASSWORD_HASH="$(./goFile hash-password)"
unset GOFILE_ADMIN_PASSWORD
```

bcrypt 对输入限制为 72 **字节**，超出时该命令会拒绝生成 hash。bcrypt hash 本身并非明文，但泄露后仍可用于离线猜测，因此也必须作为敏感凭据保护，不得提交到仓库、记录到日志、放入 URL 或公开的部署定义。

环境变量或外部密钥管理器仍是生产环境的推荐配置方式。命令行参数中的 hash、会话密钥和 Token 可能出现在 shell 历史、进程列表、服务定义、CI 日志和监控采集中。命令行模式只适合短期、本机受控调试。

PowerShell 环境变量示例：

```powershell
$env:GOFILE_ADMIN_USERNAME = "admin"
$env:GOFILE_ADMIN_PASSWORD_HASH = $hash
$env:GOFILE_SESSION_SECRET = "replace-with-a-random-secret-of-at-least-32-characters"
$env:GOFILE_API_TOKEN = "replace-with-a-separate-random-token-of-at-least-32-chars"
./goFile.exe -path "D:\data"
```

Linux/macOS 环境变量示例：

```sh
export GOFILE_ADMIN_USERNAME='admin'
# 以下值由 ./goFile hash-password 的输出提供。
export GOFILE_ADMIN_PASSWORD_HASH='<bcrypt-hash>'
export GOFILE_SESSION_SECRET='replace-with-a-random-secret-of-at-least-32-characters'
export GOFILE_API_TOKEN='replace-with-a-separate-random-token-of-at-least-32-chars'
./goFile -path /srv/files
```

完整的参数启动示例（仅适合短期本地调试，`<bcrypt-hash>` 替换为 `hash-password` 的输出）：

```powershell
./goFile.exe -path "D:\data" -admin-username "admin" -admin-password-hash "<bcrypt-hash>" -session-secret "replace-with-a-random-secret-of-at-least-32-characters" -api-token "replace-with-a-separate-random-token-of-at-least-32-chars"
```

## 参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-admin-username` | 空 | 管理员账号；显式提供时覆盖 `GOFILE_ADMIN_USERNAME` |
| `-admin-password-hash` | 空 | bcrypt 管理员密码 hash；显式提供时覆盖 `GOFILE_ADMIN_PASSWORD_HASH`，cost 为 10–12 |
| `-session-secret` | 空 | 会话签名密钥；显式提供时覆盖 `GOFILE_SESSION_SECRET`，至少 32 个字符 |
| `-api-token` | 空 | API 上传 Token；显式提供时覆盖 `GOFILE_API_TOKEN`，至少 32 个字符 |
| `-path` | 当前目录 | 受管文件根目录 |
| `-base-path` | 空 | 公开访问的 URL 子目录，例如 `/gofile`。默认空值代表站点根目录；只能使用不带末尾 `/` 的 ASCII 绝对路径 |
| `-port` | `8089` | 服务端口 |
| `-host` | `127.0.0.1` | 监听地址。要在局域网访问时需显式设置，例如 `0.0.0.0` |
| `-r` | `false` | 只读，只允许浏览、下载、预览、属性与批量 ZIP 下载 |
| `-ru` | `false` | 只读加上传，允许上传/API 上传，禁止文件管理写操作 |
| `-cookie-secure` | `false` | 为 HTTPS 反代部署设置 Secure Cookie。使用 Caddy/Nginx TLS 时必须启用 |
| `-allow-insecure-lan` | `false` | 显式允许在非 loopback 地址以 HTTP 运行且不使用 Secure Cookie，极不安全，仅限临时受控调试 |

## 访问与 HTTPS

默认服务只绑定本机：`http://127.0.0.1:8089`。若显式使用 `-host 0.0.0.0` 或局域网 IP，程序会拒绝使用非 Secure 会话 Cookie。请通过 Caddy 或 Nginx 终止 TLS，并同时使用 `-cookie-secure`：

```sh
./goFile -host 127.0.0.1 -cookie-secure -base-path /gofile -path /srv/files
```

`-base-path` 是浏览器实际访问的公开路径，而不是由请求头推断的代理信息。例如公开地址为 `https://files.example.com/gofile/` 时使用 `-base-path /gofile`。反代可以保留该前缀后转发，也可以在转发前剥离该前缀，两种方式均可使用；goFile 不信任 `X-Forwarded-Prefix` 等请求头。会话 Cookie 会自动限定在该路径，不需要额外配置 `proxy_cookie_path`。

然后由 HTTPS 反代转发到本机端口。若在非 loopback 地址直接运行 HTTP，必须额外显式传入 `-allow-insecure-lan`，这会让登录密码、会话与文件传输可被网络窃听，不适用于生产环境。预览仅以纯文本提供安全文本格式，HTML、SVG、PDF、图片和其他活跃或二进制内容会强制下载，避免上传内容在已登录管理域执行。

## 使用 systemd 部署

以下示例适用于 Debian/Ubuntu 等使用 systemd 的 Linux 主机。goFile 应仅监听本机，由 Caddy 或 Nginx 提供 HTTPS 反向代理。不要在生产环境使用 `-allow-insecure-lan`，也不要把密码 hash、会话密钥或 API Token 直接写在 `ExecStart=` 中。

### 1. 安装二进制、服务账户和数据目录

将已验证的 Linux 发布二进制安装到 root 管理的位置，创建不可登录的服务账户，以及仅供服务使用的数据目录：

```sh
install -o root -g root -m 0755 /opt/goFile/goFile /usr/local/bin/goFile
useradd --system --home /var/lib/gofile --shell /usr/sbin/nologin gofile
install -d -o gofile -g gofile -m 0750 /srv/gofile/data
install -d -o root -g root -m 0700 /etc/gofile
```

`-path` 指向的目录必须已存在，且 `gofile` 用户必须有读写权限。服务重启会清除内存中的登录会话、批量选择令牌和下载令牌，用户需要重新登录。

### 2. 创建 root 专用凭据文件

先在真实终端中生成管理员密码 hash，并生成两份独立随机值：

```sh
sudo -u gofile /usr/local/bin/goFile hash-password
openssl rand -hex 32
openssl rand -hex 32
```

创建 `/etc/gofile/gofile.env`：

```sh
install -o root -g root -m 0600 /dev/null /etc/gofile/gofile.env
nano /etc/gofile/gofile.env
```

填入以下四项，替换示例值：

```ini
GOFILE_ADMIN_USERNAME=admin
GOFILE_ADMIN_PASSWORD_HASH=$2a$10$replace-with-the-complete-bcrypt-hash
GOFILE_SESSION_SECRET=replace-with-the-first-openssl-output
GOFILE_API_TOKEN=replace-with-the-second-openssl-output
```

这是 systemd 的 `EnvironmentFile=`，不是 Shell 脚本。bcrypt hash 中的 `$` 必须原样保留，不要反斜杠转义、不要使用 `source` 加载，也不要把它复制到 `ExecStart=`。这样可避免 systemd 解析命令行时损坏 hash。拥有 root、systemd 管理权限或等效主机管理权限的主体仍可能访问运行时凭据，这些权限应视为敏感凭据访问权限。

### 3. 创建服务单元

创建 `/etc/systemd/system/gofile.service`：

```ini
[Unit]
Description=goFile Web File Manager
After=network.target

[Service]
Type=simple
User=gofile
Group=gofile
WorkingDirectory=/var/lib/gofile
StateDirectory=gofile
EnvironmentFile=/etc/gofile/gofile.env

ExecStart=/usr/local/bin/goFile -path /srv/gofile/data -host 127.0.0.1 -port 8089 -cookie-secure -base-path /gofile
# 若公开地址是 https://files.example.com/，删除上面命令中的 -base-path /gofile。

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
ReadWritePaths=/srv/gofile/data /var/lib/gofile

[Install]
WantedBy=multi-user.target
```

这里对应的公开地址为 `https://files.example.com/gofile/`。反代可以保留或剥离 `/gofile` 前缀。若使用站点根路径，移除 `-base-path /gofile`，并同时在反代配置中使用根路径。

### 4. 验证、启动与排错

```sh
systemd-analyze verify /etc/systemd/system/gofile.service
systemctl daemon-reload
systemctl enable --now gofile
systemctl status gofile --no-pager
journalctl -u gofile -f
```

若启动失败，先使用 `journalctl -u gofile -n 100 --no-pager` 查看完整错误。不要通过把敏感参数重新加入 `ExecStart=` 来排错。确认本机端口响应后，再通过 Caddy/Nginx 的 HTTPS 地址登录。

## 使用 Docker Compose 部署

仓库提供 [compose.yaml](compose.yaml)，用于**本地构建**安全镜像，不会发布或拉取官方镜像。默认容器以非 root 的 `10001:10001` 用户运行，数据持久化于命名卷 `gofile-data`，服务端口只发布到宿主机 `127.0.0.1:8089`。因此仍须使用宿主机上的 Caddy/Nginx 提供 HTTPS 访问。

### 1. 准备目录与凭据

```sh
mkdir -p /opt/gofile
cd /opt/gofile
# 将项目源码或发布源码包放在此目录，再执行后续命令。
```

用已安装的 `goFile` 二进制运行 `hash-password` 生成 hash。可先复制仓库中的 [gofile.env.example](gofile.env.example) 为 `gofile.env`，再创建 root 专用文件。`gofile.env` 已被 `.gitignore` 和 `.dockerignore` 排除，绝不能提交：

```sh
/usr/local/bin/goFile hash-password
cp gofile.env.example gofile.env
chown root:root gofile.env
chmod 0600 gofile.env
nano gofile.env
```

填入四个必要值。为避免 Compose 插值 bcrypt hash 中的 `$`，用单引号包围该值：

```ini
GOFILE_ADMIN_USERNAME=admin
GOFILE_ADMIN_PASSWORD_HASH='$2a$10$replace-with-the-complete-bcrypt-hash'
GOFILE_SESSION_SECRET=replace-with-a-random-secret-at-least-32-characters
GOFILE_API_TOKEN=replace-with-a-separate-random-token-at-least-32-characters
```

当前 `compose.yaml` 使用标准 Compose `env_file` 语法。Compose 会移除单引号，并把其中的 `$` 作为字面量传给容器。不要在 `compose.yaml` 中直接写入秘密。不要使用 `docker compose config` 的输出、`docker inspect` 输出或 shell 历史共享真实凭据。Docker daemon/API/socket 的管理权限可读取容器环境变量，应当像 root 权限一样保护。

### 2. 构建、检查并启动

```sh
docker compose build
docker compose config
docker compose up -d
docker compose ps
docker compose logs -f gofile
```

容器镜像默认运行以下等效参数：

```text
-host 0.0.0.0 -port 8089 -path /data -cookie-secure
```

`-host 0.0.0.0` 仅用于容器网络内部可达，Compose 仍通过 `127.0.0.1:8089:8089` 将端口限制在宿主机本机。由于 Cookie 使用 `Secure`，浏览器必须通过 HTTPS 反向代理登录，直接用 HTTP 打开 `http://127.0.0.1:8089` 不是可用的生产登录入口。

### 3. 配置子目录访问

若公开地址为 `https://files.example.com/gofile/`，在 `compose.yaml` 的 `gofile` 服务内增加完整 `command:`。Docker 覆盖 `CMD` 时会替换全部默认参数，不能只追加 `-base-path`：

```yaml
    command:
      - -host
      - 0.0.0.0
      - -port
      - "8089"
      - -path
      - /data
      - -cookie-secure
      - -base-path
      - /gofile
```

保存后重建或重新创建容器：

```sh
docker compose up -d --build
```

反代可保留或剥离 `/gofile` 前缀，goFile 支持两种方式。站点根目录部署时不要设置 `-base-path`。

### 4. 数据卷、升级和维护

命名卷 `gofile-data` 在容器重建后保留文件。停止服务但保留数据：

```sh
docker compose down
docker compose build --pull
docker compose up -d
docker compose logs --tail=100 gofile
```

不要运行 `docker compose down -v`，除非确定要删除全部受管文件。

如需改用宿主机目录，而不是命名卷，把 Compose 中的卷改为：

```yaml
    volumes:
      - /srv/gofile/data:/data
```

宿主机目录必须对容器的 `10001:10001` 用户可写，例如：

```sh
install -d -o 10001 -g 10001 -m 0750 /srv/gofile/data
```

若必须使用其他宿主机 UID/GID，可在构建时对齐镜像用户：

```sh
docker compose build --build-arg GOFILE_UID=$(id -u) --build-arg GOFILE_GID=$(id -g)
```

Compose 使用只读根文件系统、受限 `/tmp`、移除 Linux capabilities、禁止新增特权、PID 和内存限制。`/data` 卷与 `/tmp` 是仅有的预期可写位置。若并发上传分片或批量 ZIP 较大，请根据负载增大 `compose.yaml` 中 `/tmp` 的 `size=256m` 及 `mem_limit`。

## API 上传

脚本只允许使用独立 Token 调用上传接口，Token 不授予浏览、下载或文件管理权限。以下示例对应 `-base-path /gofile`，默认根目录部署时去掉 `/gofile`：

```sh
curl -H "Authorization: Bearer $GOFILE_API_TOKEN" \
  -F "path=logs" \
  -F "file=@/path/to/app.log" \
  http://127.0.0.1:8089/gofile/api/upload
```

## 多选规则

多选仅作用于当前目录页面的直接子项，最大 100 项。切换目录、刷新或未来的分页/筛选都会清空选择。

- 批量移动和复制会先验证全部条目、目标目录、同名冲突和安全边界。Windows 上批量移动要求源和目标位于同一磁盘卷，任一项无效或冲突时整批不会开始。
- 批量删除先进行全量预检。文件系统在执行删除时遇到不可预期 I/O 错误，页面会如实报告当前结果，不能承诺跨多个路径的崩溃级事务。
- 批量 ZIP 绑定当前登录会话、仅可使用一次，会先在系统临时目录完成安全校验和打包，再下载。不会在受管目录生成 ZIP 文件。

## 开发

```sh
go vet ./...
go test ./...
go build ./...
```

发布前应在 Linux 与 Windows 运行测试。受管目录不应包含指向受管根外部的符号链接，goFile 会拒绝对链接和特殊文件进行管理或归档。
