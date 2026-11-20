// Package sealedfile 实现单文件加密格式（.naisho）。
//
// 一个 .naisho 文件保存一篇文本笔记，用密码库的 dataKey（AES-256-GCM）加密，
// 只能由同一个密码库（或输入同一恢复密钥的 NaishoMemo）打开。
//
// 文件布局：
//
//	offset  size  内容
//	0       8     魔数 "NAISHOMO"
//	8       1     格式版本（当前为 1）
//	9       1     密钥类型（1 = 密码库 dataKey）
//	10      2     保留，必须为 0
//	12      12    GCM nonce
//	24      ...   AES-256-GCM 密文 + 16 字节认证标签
//
// 前 12 字节文件头作为 GCM 的附加认证数据（AAD），被篡改即无法解密。
// 明文是 Document 的 JSON。payload 版本 2 起可以包含图片附件。
package sealedfile

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"naishomemo/internal/fsutil"
	"naishomemo/internal/markdown"
)

const (
	// Extension 是加密文件的扩展名。
	Extension = ".naisho"

	magic          = "NAISHOMO"
	formatVersion  = byte(1)
	keyKindVault   = byte(1)
	headerSize     = 12
	nonceSize      = 12
	keySize        = 32
	payloadVersion = 2

	// MaxPlaintextSize 是可加密的原始文本文件大小上限。
	MaxPlaintextSize = 16 << 20
	// MaxAttachmentsSize 是一个加密文件中图片附件的总大小上限。
	MaxAttachmentsSize = 100 << 20
	// MaxSealedSize 是 .naisho 文件的大小上限：给 JSON 转义、base64 膨胀和文件头留出余量。
	MaxSealedSize = MaxPlaintextSize*2 + MaxAttachmentsSize*4/3 + 1<<20
	maxSealedSize = MaxSealedSize
)

var (
	ErrNotSealedFile      = errors.New("不是 NaishoMemo 加密文件")
	ErrUnsupportedVersion = errors.New("加密文件版本过新，请升级 NaishoMemo")
	ErrWrongKey           = errors.New("无法解密：该文件不属于当前密码库，或文件已损坏")
	ErrInvalidKey         = errors.New("密钥长度不正确")
	ErrAlreadySealed      = errors.New("该文件已经是 NaishoMemo 加密文件")
	ErrNotRegularFile     = errors.New("只能加密普通文件")
	ErrTooLarge           = fmt.Errorf("文件过大，最大支持 %d MB", MaxPlaintextSize>>20)
	ErrEmptyFile          = errors.New("文件为空")
	ErrNotText            = errors.New("文件不是 UTF-8 文本，请先转换为 UTF-8 编码")
	ErrVerifyFailed       = errors.New("加密后校验失败，没有写出加密文件，原文件未改动")
	ErrAttachmentsTooBig  = fmt.Errorf("图片总大小超过 %d MB，无法加密导出", MaxAttachmentsSize>>20)
)

// Attachment 是加密文件里附带的一张图片。ID 是它在原笔记正文中的引用 ID。
type Attachment struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MIMEType string `json:"mimeType"`
	Data     []byte `json:"data"`
}

// Document 是加密文件里保存的内容。
type Document struct {
	Version      int    `json:"v"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	OriginalName string `json:"originalName"`
	SealedAt     string `json:"sealedAt"`
	// Attachments 是正文中引用的图片（payload 版本 2 起）。
	Attachments []Attachment `json:"attachments,omitempty"`
}

// AttachmentsSize 返回所有附件数据的总字节数。
func (d *Document) AttachmentsSize() int {
	total := 0
	for _, a := range d.Attachments {
		total += len(a.Data)
	}
	return total
}

func sameAttachments(a, b []Attachment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].MIMEType != b[i].MIMEType || !bytes.Equal(a[i].Data, b[i].Data) {
			return false
		}
	}
	return true
}

// SealResult 描述一次文件加密的结果。
type SealResult struct {
	OutputPath      string `json:"outputPath"`
	OriginalPath    string `json:"originalPath"`
	RemovedOriginal bool   `json:"removedOriginal"`
	// RemoveError 不为空表示加密文件已写好并校验通过，但删除原文件失败。
	RemoveError string `json:"removeError,omitempty"`
}

func header() []byte {
	h := make([]byte, headerSize)
	copy(h, magic)
	h[8] = formatVersion
	h[9] = keyKindVault
	return h
}

// IsSealed 判断数据是否以 .naisho 文件头开始。
func IsSealed(data []byte) bool {
	return len(data) >= len(magic) && string(data[:len(magic)]) == magic
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != keySize {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Seal 把文档加密成 .naisho 文件内容。
func Seal(key []byte, doc Document) ([]byte, error) {
	if doc.AttachmentsSize() > MaxAttachmentsSize {
		return nil, ErrAttachmentsTooBig
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	doc.Version = payloadVersion
	plaintext, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	h := header()
	out := make([]byte, 0, headerSize+nonceSize+len(plaintext)+gcm.Overhead())
	out = append(out, h...)
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, plaintext, h)
	return out, nil
}

// Open 用给定密钥解密 .naisho 文件内容。
func Open(key []byte, data []byte) (*Document, error) {
	if !IsSealed(data) || len(data) < headerSize+nonceSize {
		return nil, ErrNotSealedFile
	}
	h := data[:headerSize]
	if h[8] != formatVersion || h[9] != keyKindVault || h[10] != 0 || h[11] != 0 {
		return nil, ErrUnsupportedVersion
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := data[headerSize : headerSize+nonceSize]
	plaintext, err := gcm.Open(nil, nonce, data[headerSize+nonceSize:], h)
	if err != nil {
		return nil, ErrWrongKey
	}
	var doc Document
	if err := json.Unmarshal(plaintext, &doc); err != nil {
		return nil, ErrWrongKey
	}
	if doc.Version > payloadVersion {
		return nil, ErrUnsupportedVersion
	}
	return &doc, nil
}

// OpenWithCandidates 依次尝试多把候选密钥，返回第一把能解开的结果。
func OpenWithCandidates(keys [][]byte, data []byte) (*Document, error) {
	lastErr := ErrWrongKey
	for _, key := range keys {
		doc, err := Open(key, data)
		if err == nil {
			return doc, nil
		}
		if errors.Is(err, ErrNotSealedFile) || errors.Is(err, ErrUnsupportedVersion) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// ReadFile 读取 .naisho 文件并检查大小。
func ReadFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotRegularFile
	}
	if info.Size() > maxSealedSize {
		return nil, ErrNotSealedFile
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !IsSealed(data) {
		return nil, ErrNotSealedFile
	}
	return data, nil
}

// OpenFile 读取并解密 .naisho 文件。
func OpenFile(key []byte, path string) (*Document, error) {
	data, err := ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Open(key, data)
}

// readText 读取待加密的文本文件，去掉 UTF-8 BOM 并校验编码。
func readText(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", ErrNotRegularFile
	}
	if info.Size() > MaxPlaintextSize {
		return "", ErrTooLarge
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if IsSealed(data) {
		return "", ErrAlreadySealed
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if len(data) == 0 {
		return "", ErrEmptyFile
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", ErrNotText
	}
	return string(data), nil
}

// OutputPathFor 返回与原文件同目录、同名的 .naisho 路径；已存在时追加 " (2)"、" (3)"……
func OutputPathFor(srcPath string) (string, error) {
	dir := filepath.Dir(srcPath)
	base := filepath.Base(srcPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if stem == "" {
		stem = base
	}
	for i := 1; i < 1000; i++ {
		name := stem + Extension
		if i > 1 {
			name = fmt.Sprintf("%s (%d)%s", stem, i, Extension)
		}
		candidate := filepath.Join(dir, name)
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", errors.New("无法生成可用的文件名")
}

// WriteDocument 加密文档并原子写入 outPath，写完后重新读取解密，确认内容一致。
// 校验失败时删除写出的文件并返回 ErrVerifyFailed。
func WriteDocument(key []byte, doc Document, outPath string) error {
	sealed, err := Seal(key, doc)
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(outPath, sealed, 0600); err != nil {
		return err
	}
	written, err := OpenFile(key, outPath)
	if err != nil || written.Content != doc.Content || written.Title != doc.Title || !sameAttachments(written.Attachments, doc.Attachments) {
		_ = os.Remove(outPath)
		return ErrVerifyFailed
	}
	return nil
}

// SealTextFile 把文本文件加密为同目录下的 .naisho 文件。
//
// 步骤：读取并校验原文 → 加密 → 原子写入 → 重新读取并解密，确认与原文一致 →
// （可选）删除原文件。任何一步在删除之前失败，都不会动原文件。
func SealTextFile(key []byte, srcPath string, removeOriginal bool, now time.Time) (*SealResult, error) {
	if strings.EqualFold(filepath.Ext(srcPath), Extension) {
		return nil, ErrAlreadySealed
	}
	content, err := readText(srcPath)
	if err != nil {
		return nil, err
	}

	doc := Document{
		Title:        markdown.ExtractTitle(srcPath, content),
		Content:      content,
		OriginalName: filepath.Base(srcPath),
		SealedAt:     now.UTC().Format(time.RFC3339),
	}
	outPath, err := OutputPathFor(srcPath)
	if err != nil {
		return nil, err
	}
	if err := WriteDocument(key, doc, outPath); err != nil {
		return nil, err
	}

	result := &SealResult{OutputPath: outPath, OriginalPath: srcPath}
	if removeOriginal {
		if err := os.Remove(srcPath); err != nil {
			result.RemoveError = err.Error()
		} else {
			result.RemovedOriginal = true
		}
	}
	return result, nil
}
