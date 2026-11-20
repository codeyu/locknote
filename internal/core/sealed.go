package core

import (
	"errors"
	"time"

	"locknote/internal/notes"
	"locknote/internal/sealedfile"
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

// SealTextFile 用当前密码库的 dataKey 把文本文件加密为同目录下的 .locknote 文件。
// removeOriginal 为 true 时，在加密文件写入并校验通过后删除原文件。
func (c *Core) SealTextFile(path string, removeOriginal bool) (*sealedfile.SealResult, error) {
	key, err := c.vaultKeyCopy()
	if err != nil {
		return nil, err
	}
	defer wipe(key)
	return sealedfile.SealTextFile(key, path, removeOriginal, time.Now())
}

// OpenSealedFile 用当前密码库的 dataKey 解密 .locknote 文件。
func (c *Core) OpenSealedFile(path string) (*sealedfile.Document, error) {
	key, err := c.vaultKeyCopy()
	if err != nil {
		return nil, err
	}
	defer wipe(key)
	return sealedfile.OpenFile(key, path)
}

// OpenSealedFileWithRecoveryKey 用恢复密钥解密 .locknote 文件，
// 用于打开由另一个密码库（同一恢复密钥）加密的文件。
func (c *Core) OpenSealedFileWithRecoveryKey(path, displayKey string) (*sealedfile.Document, error) {
	if !c.IsUnlocked() {
		return nil, errVaultLocked
	}
	data, err := sealedfile.ReadFile(path)
	if err != nil {
		return nil, err
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
func (c *Core) ImportSealedDocument(doc *sealedfile.Document) (*notes.Note, error) {
	if doc == nil {
		return nil, errors.New("没有可导入的内容")
	}
	if !c.IsUnlocked() {
		return nil, errVaultLocked
	}
	return c.noteService.Create(doc.Title, doc.Content)
}
