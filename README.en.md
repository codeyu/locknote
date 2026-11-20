# NaishoMemo

[中文](./README.md) | English

An offline-first, encrypted desktop note app for Windows, macOS, and Linux. Everything stays on your computer, protected by a master password and a recovery key.

NaishoMemo is a modified version of [LockNote](https://github.com/JackyZhang8/locknote) (MIT license).

## Features

- **Local encryption**: note bodies, titles, todos, images, tag names, notebook names, and attachment file names are encrypted with AES-256-GCM using keys derived with Argon2id
- **Master password + recovery key**: at least 8 characters, with a one-click 5-word random passphrase; the 16-character random recovery key can reset the master password
- **PIN quick unlock**: unlock with 4–12 digits after auto-lock; the master password is required after a restart or 5 wrong PINs
- **Auto-lock**: after idle time, on minimize, and after system sleep
- **`.naisho` encrypted files**: encrypt a `.txt` / `.md` file or a single note (with images) into one file that can be double-clicked, dropped into the window, or imported
- **Drop to import**: drop one or more Markdown / TXT files into the window to create notes; UTF-8, GBK, Big5, and more are detected
- **Markdown editor**: edit, preview, split view, outline, Mermaid diagrams, images
- **Todos, calendar, tags, notebooks, smart views, history, trash**
- **Full backups**: encrypted files plus a database snapshot; restoring keeps your current master password, PIN, and settings

## Where data is stored

Everything lives in the `.naishomemo` folder in your home directory:

| Platform | Path |
| --- | --- |
| Windows | `C:\Users\<user>\.naishomemo` |
| macOS / Linux | `~/.naishomemo` |

To back up, copy the whole folder or use Settings → Backup → Create backup.

## Development

Requires Go 1.24+, Node.js 20+, and the Wails CLI v2:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
wails dev
```

Build:

```bash
wails build
```

Run the same checks as CI before committing:

```bash
go vet ./internal/... .
go test ./internal/... .

cd frontend
npx tsc --noEmit
npm test
```

## License

MIT. See [LICENSE](./LICENSE), which keeps the original LockNote copyright notice.
