# NaishoMemo

中文 | [English](./README.en.md)

离线优先的桌面加密笔记软件，支持 Windows、macOS 和 Linux。所有内容只保存在本机，用主密码和恢复密钥保护。

NaishoMemo 基于 [LockNote](https://github.com/JackyZhang8/locknote)（MIT 许可）修改而来。

## 功能

- **本地加密**：笔记正文、标题、待办、图片，以及标签名、笔记本名、附件文件名都用 AES-256-GCM 加密，密钥由 Argon2id 派生
- **主密码 + 恢复密钥**：主密码至少 8 位，可一键生成 5 个单词组成的随机短语；恢复密钥为 16 位随机字符，可重置主密码
- **PIN 快速解锁**：自动锁定后用 4–12 位数字解锁；程序重启或输错 5 次后需要主密码
- **自动锁定**：闲置超时、最小化、系统休眠后自动锁定
- **加密文件 `.naisho`**：把 `.txt` / `.md` 文件或单篇笔记（含图片）加密成单个文件，可双击打开、拖入窗口查看或导入
- **拖入导入**：把一个或多个 Markdown / TXT 文件拖进窗口即新建笔记，自动识别 UTF-8、GBK、Big5 等编码
- **Markdown 编辑器**：编辑、预览、分屏，目录，Mermaid 流程图，图片
- **待办、日历、标签、笔记本、智能视图、历史版本、回收站**
- **完整备份**：备份包含加密文件和数据库快照；恢复时保留当前主密码、PIN 和设置

## 数据存放位置

所有数据都在用户目录下的 `.naishomemo` 文件夹中：

| 平台 | 路径 |
| --- | --- |
| Windows | `C:\Users\<用户名>\.naishomemo` |
| macOS / Linux | `~/.naishomemo` |

备份时请复制整个文件夹，或使用“设置 → 备份 → 创建备份”。

## 开发

需要 Go 1.24+、Node.js 20+ 和 Wails CLI v2：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
wails dev
```

打包：

```bash
wails build
```

提交前运行与 CI 相同的检查：

```bash
go vet ./internal/... .
go test ./internal/... .

cd frontend
npx tsc --noEmit
npm test
```

## 许可

MIT。见 [LICENSE](./LICENSE)，其中保留了原项目 LockNote 的版权声明。
