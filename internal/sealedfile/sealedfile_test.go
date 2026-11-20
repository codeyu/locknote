package sealedfile

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var (
	testKey  = bytes.Repeat([]byte{7}, 32)
	otherKey = bytes.Repeat([]byte{8}, 32)
	testNow  = time.Date(2026, 11, 20, 6, 0, 0, 0, time.UTC)
)

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestSealOpenRoundTrip(t *testing.T) {
	doc := Document{Title: "密码提示", Content: "银行卡：生日倒序\nAPI key: sk-test", OriginalName: "note.txt", SealedAt: "2026-11-20T06:00:00Z"}
	data, err := Seal(testKey, doc)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if !IsSealed(data) {
		t.Fatal("sealed data should start with the magic header")
	}
	if bytes.Contains(data, []byte("sk-test")) || bytes.Contains(data, []byte("note.txt")) {
		t.Fatal("sealed data must not contain plaintext")
	}
	got, err := Open(testKey, data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got.Title != doc.Title || got.Content != doc.Content || got.OriginalName != doc.OriginalName || got.SealedAt != doc.SealedAt {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if got.Version != payloadVersion {
		t.Fatalf("version = %d, want %d", got.Version, payloadVersion)
	}
}

func TestSealUsesFreshNonce(t *testing.T) {
	doc := Document{Title: "a", Content: "same"}
	a, _ := Seal(testKey, doc)
	b, _ := Seal(testKey, doc)
	if bytes.Equal(a, b) {
		t.Fatal("two seals of the same document should differ")
	}
}

func TestOpenRejectsWrongKeyTamperingAndForeignFiles(t *testing.T) {
	data, err := Seal(testKey, Document{Title: "t", Content: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(otherKey, data); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("wrong key: err = %v", err)
	}

	tamperedBody := append([]byte(nil), data...)
	tamperedBody[len(tamperedBody)-1] ^= 0xff
	if _, err := Open(testKey, tamperedBody); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("tampered body: err = %v", err)
	}

	tamperedHeader := append([]byte(nil), data...)
	tamperedHeader[9] = 9
	if _, err := Open(testKey, tamperedHeader); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("tampered header: err = %v", err)
	}

	newer := append([]byte(nil), data...)
	newer[8] = formatVersion + 1
	if _, err := Open(testKey, newer); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("newer version: err = %v", err)
	}

	for _, foreign := range [][]byte{nil, []byte("hello"), []byte("NAISHOMO"), []byte("PK\x03\x04zip")} {
		if _, err := Open(testKey, foreign); !errors.Is(err, ErrNotSealedFile) {
			t.Fatalf("foreign %q: err = %v", foreign, err)
		}
	}

	if _, err := Open([]byte("short"), data); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("short key: err = %v", err)
	}
}

func TestOpenWithCandidates(t *testing.T) {
	data, _ := Seal(testKey, Document{Title: "t", Content: "c"})
	doc, err := OpenWithCandidates([][]byte{otherKey, testKey}, data)
	if err != nil || doc.Content != "c" {
		t.Fatalf("candidates: doc=%v err=%v", doc, err)
	}
	if _, err := OpenWithCandidates([][]byte{otherKey}, data); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("no matching key: err = %v", err)
	}
	if _, err := OpenWithCandidates(nil, data); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("no keys: err = %v", err)
	}
	if _, err := OpenWithCandidates([][]byte{testKey}, []byte("plain")); !errors.Is(err, ErrNotSealedFile) {
		t.Fatalf("plain file: err = %v", err)
	}
}

func TestSealTextFileWritesVerifiesAndRemovesOriginal(t *testing.T) {
	dir := t.TempDir()
	src := writeFile(t, dir, "note.txt", []byte("\xef\xbb\xbf# 账号\nuser: alice\n"))

	result, err := SealTextFile(testKey, src, true, testNow)
	if err != nil {
		t.Fatalf("seal file: %v", err)
	}
	if result.OutputPath != filepath.Join(dir, "note.naisho") {
		t.Fatalf("output = %s", result.OutputPath)
	}
	if !result.RemovedOriginal || result.RemoveError != "" {
		t.Fatalf("original should be removed: %+v", result)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("original still exists: %v", err)
	}
	if info, err := os.Stat(result.OutputPath); err != nil {
		t.Fatal(err)
	} else if os.PathSeparator == '/' && info.Mode().Perm() != 0600 {
		t.Fatalf("perm = %v, want 0600", info.Mode().Perm())
	}

	doc, err := OpenFile(testKey, result.OutputPath)
	if err != nil {
		t.Fatalf("open written file: %v", err)
	}
	if doc.Content != "# 账号\nuser: alice\n" {
		t.Fatalf("content = %q (BOM should be stripped)", doc.Content)
	}
	if doc.Title != "账号" || doc.OriginalName != "note.txt" || doc.SealedAt != "2026-11-20T06:00:00Z" {
		t.Fatalf("metadata = %+v", doc)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected only the sealed file, found %d entries (temp file left?)", len(entries))
	}
}

func TestSealTextFileKeepsOriginalWhenAsked(t *testing.T) {
	dir := t.TempDir()
	src := writeFile(t, dir, "keep.md", []byte("hello"))
	result, err := SealTextFile(testKey, src, false, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if result.RemovedOriginal {
		t.Fatal("original should be kept")
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("original missing: %v", err)
	}
	doc, _ := OpenFile(testKey, result.OutputPath)
	if doc.Title != "keep" {
		t.Fatalf("title should fall back to the file name, got %q", doc.Title)
	}
}

func TestSealTextFileDoesNotOverwriteExistingOutput(t *testing.T) {
	dir := t.TempDir()
	existing := writeFile(t, dir, "note.naisho", []byte("previous"))
	src := writeFile(t, dir, "note.txt", []byte("new"))
	result, err := SealTextFile(testKey, src, false, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if result.OutputPath != filepath.Join(dir, "note (2).naisho") {
		t.Fatalf("output = %s", result.OutputPath)
	}
	if data, _ := os.ReadFile(existing); string(data) != "previous" {
		t.Fatal("existing .naisho file was overwritten")
	}
}

func TestSealTextFileRejectsBadInputsAndKeepsOriginal(t *testing.T) {
	dir := t.TempDir()
	sealedData, _ := Seal(testKey, Document{Content: "x"})
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"empty.txt", nil, ErrEmptyFile},
		{"bom-only.txt", []byte{0xef, 0xbb, 0xbf}, ErrEmptyFile},
		{"gbk.txt", []byte{0xc4, 0xe3, 0xba, 0xc3}, ErrNotText},
		{"binary.txt", []byte("ab\x00cd"), ErrNotText},
		{"renamed.txt", sealedData, ErrAlreadySealed},
		{"already.naisho", []byte("text"), ErrAlreadySealed},
	}
	for _, tc := range cases {
		src := writeFile(t, dir, tc.name, tc.data)
		if _, err := SealTextFile(testKey, src, true, testNow); !errors.Is(err, tc.want) {
			t.Fatalf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
		if _, err := os.Stat(src); err != nil {
			t.Fatalf("%s: original must be kept on error: %v", tc.name, err)
		}
	}
	if _, err := SealTextFile(testKey, dir, true, testNow); !errors.Is(err, ErrNotRegularFile) {
		t.Fatalf("directory: err = %v", err)
	}
	if _, err := SealTextFile(testKey, filepath.Join(dir, "missing.txt"), true, testNow); err == nil {
		t.Fatal("missing file should fail")
	}
	big := writeFile(t, dir, "big.txt", []byte(strings.Repeat("a", MaxPlaintextSize+1)))
	if _, err := SealTextFile(testKey, big, true, testNow); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("big file: err = %v", err)
	}
	src := writeFile(t, dir, "badkey.txt", []byte("hello"))
	if _, err := SealTextFile([]byte("short"), src, true, testNow); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("bad key: err = %v", err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatal("original must be kept when the key is invalid")
	}
}

func TestReadFileRejectsNonSealed(t *testing.T) {
	dir := t.TempDir()
	plain := writeFile(t, dir, "plain.naisho", []byte("not really"))
	if _, err := OpenFile(testKey, plain); !errors.Is(err, ErrNotSealedFile) {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteDocument(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "exported.naisho")
	doc := Document{Title: "导出", Content: "body", SealedAt: "2026-11-20T06:00:00Z"}
	if err := WriteDocument(testKey, doc, out); err != nil {
		t.Fatal(err)
	}
	got, err := OpenFile(testKey, out)
	if err != nil || got.Title != "导出" || got.Content != "body" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if err := WriteDocument([]byte("bad"), doc, filepath.Join(dir, "x.naisho")); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("bad key: err = %v", err)
	}
}

func TestAttachmentsRoundTrip(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nfake-image-bytes")
	doc := Document{
		Title:   "带图片",
		Content: "看图 ![a](naisho-attachment://img-1)",
		Attachments: []Attachment{
			{ID: "img-1", Name: "a.png", MIMEType: "image/png", Data: png},
		},
	}
	out := filepath.Join(t.TempDir(), "img.naisho")
	if err := WriteDocument(testKey, doc, out); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(out)
	if bytes.Contains(raw, []byte("fake-image-bytes")) || bytes.Contains(raw, []byte("a.png")) {
		t.Fatal("attachment data or name leaked in plaintext")
	}
	got, err := OpenFile(testKey, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].ID != "img-1" || got.Attachments[0].Name != "a.png" ||
		got.Attachments[0].MIMEType != "image/png" || !bytes.Equal(got.Attachments[0].Data, png) {
		t.Fatalf("attachments = %+v", got.Attachments)
	}
}

func TestVersion1PayloadStillOpens(t *testing.T) {
	// 手工构造一个 payload 版本 1（没有 attachments 字段）的文件
	gcm, _ := newGCM(testKey)
	h := header()
	nonce := make([]byte, nonceSize)
	plaintext := []byte(`{"v":1,"title":"旧格式","content":"body","originalName":"x.txt","sealedAt":""}`)
	data := append(append(append([]byte{}, h...), nonce...), gcm.Seal(nil, nonce, plaintext, h)...)
	doc, err := Open(testKey, data)
	if err != nil || doc.Title != "旧格式" || doc.Content != "body" || len(doc.Attachments) != 0 {
		t.Fatalf("v1 doc = %+v, %v", doc, err)
	}
}

func TestAttachmentsSizeLimit(t *testing.T) {
	doc := Document{Content: "x", Attachments: []Attachment{{ID: "big", MIMEType: "image/png", Data: make([]byte, MaxAttachmentsSize+1)}}}
	if _, err := Seal(testKey, doc); !errors.Is(err, ErrAttachmentsTooBig) {
		t.Fatalf("err = %v", err)
	}
}
