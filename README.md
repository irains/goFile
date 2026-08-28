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

goFile 默认强制登录。启动前必须配置以下环境变量：

| 变量 | 说明 |
| --- | --- |
| `GOFILE_ADMIN_USERNAME` | 唯一管理员账号 |
| `GOFILE_ADMIN_PASSWORD` | 管理员密码 |
| `GOFILE_SESSION_SECRET` | 会话签名密钥，至少 32 个字符，建议使用随机 32 字节以上值 |
| `GOFILE_API_TOKEN` | 脚本上传 Token，至少 32 个字符，建议使用随机 32 字节以上值 |

PowerShell 示例：

```powershell
$env:GOFILE_ADMIN_USERNAME = "admin"
$env:GOFILE_ADMIN_PASSWORD = "use-a-long-unique-password"
$env:GOFILE_SESSION_SECRET = "replace-with-a-random-secret-of-at-least-32-characters"
$env:GOFILE_API_TOKEN = "replace-with-a-separate-random-token-of-at-least-32-chars"
./goFile.exe -path "D:\data"
```

Linux/macOS 示例：

```sh
export GOFILE_ADMIN_USERNAME='admin'
export GOFILE_ADMIN_PASSWORD='use-a-long-unique-password'
export GOFILE_SESSION_SECRET='replace-with-a-random-secret-of-at-least-32-characters'
export GOFILE_API_TOKEN='replace-with-a-separate-random-token-of-at-least-32-chars'
./goFile -path /srv/files
```

不要把真实凭据提交到仓库、写进 shell 历史或放进 URL。

## 参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-path` | 当前目录 | 受管文件根目录 |
| `-port` | `8089` | 服务端口 |
| `-host` | `127.0.0.1` | 监听地址。要在局域网访问时需显式设置，例如 `0.0.0.0` |
| `-r` | `false` | 只读，只允许浏览、下载、预览、属性与批量 ZIP 下载 |
| `-ru` | `false` | 只读加上传，允许上传/API 上传，禁止文件管理写操作 |
| `-cookie-secure` | `false` | 为 HTTPS 反代部署设置 Secure Cookie。使用 Caddy/Nginx TLS 时必须启用 |
| `-allow-insecure-lan` | `false` | 显式允许在非 loopback 地址以 HTTP 运行且不使用 Secure Cookie，极不安全，仅限临时受控调试 |

## 访问与 HTTPS

默认服务只绑定本机：`http://127.0.0.1:8089`。若显式使用 `-host 0.0.0.0` 或局域网 IP，程序会拒绝使用非 Secure 会话 Cookie。请通过 Caddy 或 Nginx 终止 TLS，并同时使用 `-cookie-secure`：

```sh
./goFile -host 127.0.0.1 -cookie-secure -path /srv/files
```

然后由 HTTPS 反代转发到本机端口。若在非 loopback 地址直接运行 HTTP，必须额外显式传入 `-allow-insecure-lan`，这会让登录密码、会话与文件传输可被网络窃听，不适用于生产环境。预览仅以纯文本提供安全文本格式，HTML、SVG、PDF、图片和其他活跃或二进制内容会强制下载，避免上传内容在已登录管理域执行。

## API 上传

脚本只允许使用独立 Token 调用上传接口，Token 不授予浏览、下载或文件管理权限：

```sh
curl -H "Authorization: Bearer $GOFILE_API_TOKEN" \
  -F "path=logs" \
  -F "file=@/path/to/app.log" \
  http://127.0.0.1:8089/api/upload
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
