package database

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// fakeCipher 是测试用的可逆“加密”：加前缀并翻转字节。
type fakeCipher struct{}

func (fakeCipher) Encrypt(p []byte) ([]byte, error) {
	out := append([]byte("ENC:"), p...)
	for i, j := 4, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (fakeCipher) Decrypt(c []byte) ([]byte, error) {
	if !bytes.HasPrefix(c, []byte("ENC:")) {
		return nil, errors.New("bad ciphertext")
	}
	out := append([]byte(nil), c[4:]...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func newFieldTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := New(filepath.Join(t.TempDir(), "locknote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestEncryptedFieldsAreNotStoredInPlaintext(t *testing.T) {
	db := newFieldTestDB(t)
	db.SetFieldCipher(fakeCipher{})
	now := time.Now()

	if err := db.CreateTag(&Tag{ID: "t1", Name: "银行", Color: "#fff"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateNotebook(&Notebook{ID: "nb1", Name: "私人账户", Icon: "📓", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateSmartView(&SmartView{ID: "sv1", Name: "密码相关", Icon: "🔍", FilterJSON: `{"query":"password"}`}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateAttachment(&AttachmentMeta{ID: "a1", OriginalName: "护照扫描.png", MIMEType: "image/png", Size: 1, CipherPath: "attachments/a1.enc", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	var raw string
	for _, q := range []string{
		`SELECT name FROM tags`,
		`SELECT name FROM notebooks`,
		`SELECT name || filter_json FROM smart_views`,
		`SELECT original_name FROM attachments`,
	} {
		if err := db.db.QueryRow(q).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"银行", "私人账户", "密码相关", "password", "护照"} {
			if bytes.Contains([]byte(raw), []byte(secret)) {
				t.Fatalf("%s leaked plaintext %q (raw=%q)", q, secret, raw)
			}
		}
	}

	tags, err := db.ListTags()
	if err != nil || len(tags) != 1 || tags[0].Name != "银行" {
		t.Fatalf("tags = %+v, %v", tags, err)
	}
	nb, err := db.GetNotebook("nb1")
	if err != nil || nb.Name != "私人账户" {
		t.Fatalf("notebook = %+v, %v", nb, err)
	}
	sv, err := db.GetSmartView("sv1")
	if err != nil || sv.Name != "密码相关" || sv.FilterJSON != `{"query":"password"}` {
		t.Fatalf("smart view = %+v, %v", sv, err)
	}
	att, err := db.GetAttachment("a1")
	if err != nil || att.OriginalName != "护照扫描.png" {
		t.Fatalf("attachment = %+v, %v", att, err)
	}

	// 锁定后无法读取加密字段
	db.SetFieldCipher(nil)
	if _, err := db.ListTags(); !errors.Is(err, ErrFieldsLocked) {
		t.Fatalf("locked ListTags err = %v", err)
	}
}

func TestDuplicateTagNamesAreRejected(t *testing.T) {
	db := newFieldTestDB(t)
	db.SetFieldCipher(fakeCipher{})
	if err := db.CreateTag(&Tag{ID: "t1", Name: "工作", Color: "#fff"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateTag(&Tag{ID: "t2", Name: "工作", Color: "#000"}); !errors.Is(err, ErrDuplicateTagName) {
		t.Fatalf("duplicate create err = %v", err)
	}
	if err := db.CreateTag(&Tag{ID: "t3", Name: "生活", Color: "#000"}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateTag(&Tag{ID: "t3", Name: "工作", Color: "#000"}); !errors.Is(err, ErrDuplicateTagName) {
		t.Fatalf("duplicate update err = %v", err)
	}
	if err := db.UpdateTag(&Tag{ID: "t1", Name: "工作", Color: "#123"}); err != nil {
		t.Fatalf("renaming a tag to its own name should work: %v", err)
	}
	tags, _ := db.ListTags()
	if len(tags) != 2 || tags[0].Name != "工作" && tags[1].Name != "工作" {
		t.Fatalf("tags = %+v", tags)
	}
}

func TestEncryptLegacyFieldsMigratesPlaintextRows(t *testing.T) {
	db := newFieldTestDB(t)
	now := time.Now()
	// 没有加密器：按旧方式写入明文
	if err := db.CreateTag(&Tag{ID: "t1", Name: "旧标签", Color: "#fff"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateNotebook(&Notebook{ID: "nb1", Name: "旧笔记本", Icon: "📓", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.EncryptLegacyFields(); !errors.Is(err, ErrFieldsLocked) {
		t.Fatalf("migration without cipher err = %v", err)
	}

	db.SetFieldCipher(fakeCipher{})
	n, err := db.EncryptLegacyFields()
	if err != nil || n != 2 {
		t.Fatalf("migrated %d, %v; want 2 fields (tag name, notebook name)", n, err)
	}
	if n, err := db.EncryptLegacyFields(); err != nil || n != 0 {
		t.Fatalf("second migration = %d, %v; want 0", n, err)
	}
	var rawName string
	if err := db.db.QueryRow(`SELECT name FROM tags WHERE id = 't1'`).Scan(&rawName); err != nil {
		t.Fatal(err)
	}
	if rawName != "t1" {
		t.Fatalf("tag placeholder = %q, want the id", rawName)
	}
	tags, err := db.ListTags()
	if err != nil || tags[0].Name != "旧标签" {
		t.Fatalf("tags after migration = %+v, %v", tags, err)
	}
	books, err := db.ListNotebooks()
	if err != nil || books[0].Name != "旧笔记本" {
		t.Fatalf("notebooks after migration = %+v, %v", books, err)
	}
}
