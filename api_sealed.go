package main

import (
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"

	"locknote/internal/notes"
	"locknote/internal/sealedfile"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SealedFileView 是给前端展示的 .locknote 文件内容。
type SealedFileView struct {
	Path         string `json:"path"`
	FileName     string `json:"fileName"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	OriginalName string `json:"originalName"`
	SealedAt     string `json:"sealedAt"`
}

func newSealedFileView(path string, doc *sealedfile.Document) *SealedFileView {
	return &SealedFileView{
		Path:         path,
		FileName:     filepath.Base(path),
		Title:        doc.Title,
		Content:      doc.Content,
		OriginalName: doc.OriginalName,
		SealedAt:     doc.SealedAt,
	}
}

func isSealedFilePath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), sealedfile.Extension)
}

// SealTextFile 让用户选择一个文本文件，加密为同目录下的 .locknote 文件。
// 用户取消选择时返回 nil, nil。
func (a *App) SealTextFile(removeOriginal bool) (*sealedfile.SealResult, error) {
	c, err := a.coreOrErr()
	if err != nil {
		return nil, err
	}
	a.UpdateActivity()

	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择要加密的文本文件",
		Filters: []runtime.FileFilter{
			{DisplayName: "文本文件 (*.txt;*.md)", Pattern: "*.txt;*.md;*.markdown"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	return c.SealTextFile(path, removeOriginal)
}

// ChooseSealedFile 让用户选择一个 .locknote 文件，返回其路径；取消时返回空字符串。
func (a *App) ChooseSealedFile() (string, error) {
	if _, err := a.coreOrErr(); err != nil {
		return "", err
	}
	a.UpdateActivity()
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "打开 LockNote 加密文件",
		Filters: []runtime.FileFilter{
			{DisplayName: "LockNote 加密文件 (*.locknote)", Pattern: "*.locknote"},
		},
	})
}

// OpenSealedFile 用当前密码库解密指定的 .locknote 文件。
func (a *App) OpenSealedFile(path string) (*SealedFileView, error) {
	c, err := a.coreOrErr()
	if err != nil {
		return nil, err
	}
	a.UpdateActivity()
	doc, err := c.OpenSealedFile(path)
	if err != nil {
		return nil, err
	}
	return newSealedFileView(path, doc), nil
}

// OpenSealedFileWithRecoveryKey 用恢复密钥解密 .locknote 文件（来自另一个密码库时使用）。
func (a *App) OpenSealedFileWithRecoveryKey(path, recoveryKey string) (*SealedFileView, error) {
	c, err := a.coreOrErr()
	if err != nil {
		return nil, err
	}
	a.UpdateActivity()
	doc, err := c.OpenSealedFileWithRecoveryKey(path, recoveryKey)
	if err != nil {
		return nil, err
	}
	return newSealedFileView(path, doc), nil
}

// ImportSealedFile 把 .locknote 文件的内容存为密码库中的新笔记。
// recoveryKey 为空时使用当前密码库的密钥解密。
func (a *App) ImportSealedFile(path, recoveryKey string) (*notes.Note, error) {
	c, err := a.coreOrErr()
	if err != nil {
		return nil, err
	}
	a.UpdateActivity()
	var doc *sealedfile.Document
	if strings.TrimSpace(recoveryKey) == "" {
		doc, err = c.OpenSealedFile(path)
	} else {
		doc, err = c.OpenSealedFileWithRecoveryKey(path, recoveryKey)
	}
	if err != nil {
		return nil, err
	}
	return c.ImportSealedDocument(doc)
}

// maxSealedDataBase64 限制前端传入的 base64 数据大小（约 48 MB 原始数据）。
const maxSealedDataBase64 = 64 << 20

func decodeSealedData(dataBase64 string) ([]byte, error) {
	if len(dataBase64) > maxSealedDataBase64 {
		return nil, sealedfile.ErrNotSealedFile
	}
	data, err := base64.StdEncoding.DecodeString(dataBase64)
	if err != nil {
		return nil, errors.New("无法读取文件内容")
	}
	return data, nil
}

// OpenSealedFileData 解密拖入窗口的 .locknote 文件（WebView 拿不到文件路径，只能传内容）。
// recoveryKey 为空时使用当前密码库的密钥。
func (a *App) OpenSealedFileData(fileName, dataBase64, recoveryKey string) (*SealedFileView, error) {
	c, err := a.coreOrErr()
	if err != nil {
		return nil, err
	}
	a.UpdateActivity()
	data, err := decodeSealedData(dataBase64)
	if err != nil {
		return nil, err
	}
	var doc *sealedfile.Document
	if strings.TrimSpace(recoveryKey) == "" {
		doc, err = c.OpenSealedData(data)
	} else {
		doc, err = c.OpenSealedDataWithRecoveryKey(data, recoveryKey)
	}
	if err != nil {
		return nil, err
	}
	view := newSealedFileView("", doc)
	view.FileName = filepath.Base(fileName)
	return view, nil
}

// ImportSealedFileData 把拖入的 .locknote 文件内容存为新笔记。
func (a *App) ImportSealedFileData(dataBase64, recoveryKey string) (*notes.Note, error) {
	c, err := a.coreOrErr()
	if err != nil {
		return nil, err
	}
	a.UpdateActivity()
	data, err := decodeSealedData(dataBase64)
	if err != nil {
		return nil, err
	}
	var doc *sealedfile.Document
	if strings.TrimSpace(recoveryKey) == "" {
		doc, err = c.OpenSealedData(data)
	} else {
		doc, err = c.OpenSealedDataWithRecoveryKey(data, recoveryKey)
	}
	if err != nil {
		return nil, err
	}
	return c.ImportSealedDocument(doc)
}

// SealedExportResult 是加密导出笔记的结果。
type SealedExportResult struct {
	Path string `json:"path"`
	// OmittedImages 是笔记中引用、但没有包含进加密文件的图片数量。
	OmittedImages int `json:"omittedImages"`
}

// ExportNoteAsLockNote 把当前笔记加密导出为 .locknote 文件。用户取消时返回 nil, nil。
func (a *App) ExportNoteAsLockNote(noteID string) (*SealedExportResult, error) {
	c, err := a.coreOrErr()
	if err != nil {
		return nil, err
	}
	a.UpdateActivity()

	note, err := c.Notes().Get(noteID)
	if err != nil {
		return nil, err
	}
	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "加密导出笔记",
		DefaultFilename: sanitizeFileName(note.Title) + sealedfile.Extension,
		Filters: []runtime.FileFilter{
			{DisplayName: "LockNote 加密文件 (*.locknote)", Pattern: "*.locknote"},
		},
	})
	if err != nil {
		return nil, err
	}
	if savePath == "" {
		return nil, nil
	}
	if !isSealedFilePath(savePath) {
		savePath += sealedfile.Extension
	}
	if err := c.ExportNoteSealed(noteID, savePath); err != nil {
		return nil, err
	}
	return &SealedExportResult{Path: savePath, OmittedImages: len(collectAttachmentIDs(note.Content))}, nil
}

// ============ 双击 / “打开方式” 传入的 .locknote 文件 ============

// queueSealedFiles 记录系统交给本程序打开的 .locknote 文件，等前端解锁后取走。
func (a *App) queueSealedFiles(paths []string) {
	added := false
	a.pendingMu.Lock()
	for _, p := range paths {
		if p == "" || !isSealedFilePath(p) {
			continue
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		a.pendingSealed = append(a.pendingSealed, p)
		added = true
	}
	a.pendingMu.Unlock()
	if added && a.ctx != nil {
		runtime.EventsEmit(a.ctx, "sealed:pending")
	}
}

// TakePendingSealedFile 取出一个等待打开的 .locknote 文件路径；没有时返回空字符串。
func (a *App) TakePendingSealedFile() string {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	if len(a.pendingSealed) == 0 {
		return ""
	}
	p := a.pendingSealed[0]
	a.pendingSealed = a.pendingSealed[1:]
	return p
}

// onFileOpen 处理 macOS 的“用 LockNote 打开”事件。
func (a *App) onFileOpen(path string) {
	a.queueSealedFiles([]string{path})
	a.focusWindow()
}

// onSecondInstanceLaunch 在程序已运行时再次双击 .locknote 文件时调用：
// 把文件交给已运行的实例，而不是再开一个窗口。
func (a *App) onSecondInstanceLaunch(data options.SecondInstanceData) {
	a.queueSealedFiles(data.Args)
	a.focusWindow()
}

func (a *App) focusWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
}
