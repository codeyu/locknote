package core

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"naishomemo/internal/attachments"
	"naishomemo/internal/notes"
	"naishomemo/internal/sealedfile"
)

var errVaultLocked = errors.New("请先解锁")

// vaultKeyCopy 返回 dataKey 的副本，调用方用完后应调用 wipe 清零。
func (c *Core) vaultKeyCopy() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.isUnlocked || len(c.dataKey) == 0 {
		return nil, errVaultLocked
	}
	return append([]byte(nil), c.dataKey...), nil
}

func wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// SealTextFile 用当前密码库的 dataKey 把文本文件加密为同目录下的 .naisho 文件。
// removeOriginal 为 true 时，在加密文件写入并校验通过后删除原文件。
func (c *Core) SealTextFile(path string, removeOriginal bool) (*sealedfile.SealResult, error) {
	key, err := c.vaultKeyCopy()
	if err != nil {
		return nil, err
	}
	defer wipe(key)
	return sealedfile.SealTextFile(key, path, removeOriginal, time.Now())
}

// OpenSealedFile 用当前密码库的 dataKey 解密 .naisho 文件。
func (c *Core) OpenSealedFile(path string) (*sealedfile.Document, error) {
	data, err := sealedfile.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return c.OpenSealedData(data)
}

// OpenSealedData 用当前密码库的 dataKey 解密 .naisho 文件内容（例如拖入窗口的文件）。
func (c *Core) OpenSealedData(data []byte) (*sealedfile.Document, error) {
	key, err := c.vaultKeyCopy()
	if err != nil {
		return nil, err
	}
	defer wipe(key)
	return sealedfile.Open(key, data)
}

// OpenSealedFileWithRecoveryKey 用恢复密钥解密 .naisho 文件，
// 用于打开由另一个密码库（同一恢复密钥）加密的文件。
func (c *Core) OpenSealedFileWithRecoveryKey(path, displayKey string) (*sealedfile.Document, error) {
	data, err := sealedfile.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return c.OpenSealedDataWithRecoveryKey(data, displayKey)
}

// OpenSealedDataWithRecoveryKey 用恢复密钥解密 .naisho 文件内容。
func (c *Core) OpenSealedDataWithRecoveryKey(data []byte, displayKey string) (*sealedfile.Document, error) {
	if !c.IsUnlocked() {
		return nil, errVaultLocked
	}
	candidates := c.cryptoService.DataKeyCandidates(displayKey)
	defer func() {
		for _, k := range candidates {
			wipe(k)
		}
	}()
	if len(candidates) == 0 {
		return nil, errors.New("恢复密钥格式不正确")
	}
	return sealedfile.OpenWithCandidates(candidates, data)
}

// ImportSealedDocument 把解密后的文档存为密码库中的一篇新笔记。
// 文件里附带的图片会重新存为附件，正文中的图片引用改指向新附件。
func (c *Core) ImportSealedDocument(doc *sealedfile.Document) (*notes.Note, error) {
	if doc == nil {
		return nil, errors.New("没有可导入的内容")
	}
	if !c.IsUnlocked() {
		return nil, errVaultLocked
	}

	content := doc.Content
	var created []string
	cleanup := func() {
		for _, id := range created {
			_ = c.attachmentService.Delete(id)
		}
	}
	for _, a := range doc.Attachments {
		att, err := c.attachmentService.CreateImage(attachments.CreateImageInput{
			OriginalName: a.Name,
			MIMEType:     a.MIMEType,
			Data:         a.Data,
		})
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("导入图片 %s 失败: %w", a.Name, err)
		}
		created = append(created, att.ID)
		content = strings.ReplaceAll(content, "naisho-attachment://"+a.ID, "naisho-attachment://"+att.ID)
	}

	note, err := c.noteService.Create(doc.Title, content)
	if err != nil {
		cleanup()
		return nil, err
	}
	for _, id := range created {
		if err := c.attachmentService.AttachToNote(note.ID, id); err != nil {
			return note, fmt.Errorf("图片关联到笔记失败: %w", err)
		}
	}
	return note, nil
}

// attachmentRefPattern 匹配笔记正文中的图片引用，与 api.go 的 attachmentURLPattern 一致。
var attachmentRefPattern = regexp.MustCompile(`naisho-attachment://([A-Za-z0-9_-]+)`)

func attachmentRefs(content string) []string {
	seen := map[string]bool{}
	var ids []string
	for _, m := range attachmentRefPattern.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 && !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}
	return ids
}

// ExportNoteSealed 把一篇笔记连同正文里引用的图片加密导出为 .naisho 文件。
// 返回找不到（已被彻底删除）而没有包含进去的图片数量。
func (c *Core) ExportNoteSealed(noteID, outPath string) (int, error) {
	note, err := c.noteService.Get(noteID)
	if err != nil {
		return 0, err
	}
	doc := sealedfile.Document{
		Title:    note.Title,
		Content:  note.Content,
		SealedAt: time.Now().UTC().Format(time.RFC3339),
	}
	missing := 0
	total := 0
	for _, id := range attachmentRefs(note.Content) {
		meta, data, err := c.attachmentService.GetData(id)
		if err != nil {
			missing++
			continue
		}
		total += len(data)
		if total > sealedfile.MaxAttachmentsSize {
			return 0, sealedfile.ErrAttachmentsTooBig
		}
		doc.Attachments = append(doc.Attachments, sealedfile.Attachment{
			ID:       id,
			Name:     meta.OriginalName,
			MIMEType: meta.MIMEType,
			Data:     data,
		})
	}

	key, err := c.vaultKeyCopy()
	if err != nil {
		return 0, err
	}
	defer wipe(key)
	if err := sealedfile.WriteDocument(key, doc, outPath); err != nil {
		return 0, err
	}
	return missing, nil
}
