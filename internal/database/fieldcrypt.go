package database

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

// 字段级加密：标签名、笔记本名、智能视图名称与筛选条件、附件原文件名
// 以前是明文存在数据库里；现在用 dataKey 加密后存进 *_enc 列，明文列只放占位值。
//
// 规则：
//   - 设置了 FieldCipher（已解锁）时，写入一律加密；
//   - 没有 FieldCipher 时按旧方式写明文（只在测试或解锁前的内部调用中出现），
//     下次解锁时由 EncryptLegacyFields 补加密；
//   - 读取时有 *_enc 就解密，没有就用明文列（兼容旧数据）。

// FieldCipher 用当前密码库密钥加解密短字段。
type FieldCipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// ErrFieldsLocked 表示需要解密字段但密码库已锁定。
var ErrFieldsLocked = errors.New("请先解锁")

// ErrDuplicateTagName 表示标签名已存在（加密后无法再依赖数据库的唯一约束）。
var ErrDuplicateTagName = errors.New("标签名已存在")

type fieldCrypto struct {
	mu     sync.RWMutex
	cipher FieldCipher
}

// SetFieldCipher 设置（解锁时）或清除（锁定时，传 nil）字段加密器。
func (d *DB) SetFieldCipher(c FieldCipher) {
	d.fields.mu.Lock()
	d.fields.cipher = c
	d.fields.mu.Unlock()
}

func (d *DB) fieldCipher() FieldCipher {
	d.fields.mu.RLock()
	defer d.fields.mu.RUnlock()
	return d.fields.cipher
}

// sealField 返回要写入的（明文列, 密文列）。没有加密器时返回 (原文, nil)。
func (d *DB) sealField(value, placeholder string) (string, []byte, error) {
	c := d.fieldCipher()
	if c == nil {
		return value, nil, nil
	}
	enc, err := c.Encrypt([]byte(value))
	if err != nil {
		return "", nil, err
	}
	return placeholder, enc, nil
}

// openField 读取字段：有密文就解密，否则返回明文列。
func (d *DB) openField(plain string, enc []byte) (string, error) {
	if len(enc) == 0 {
		return plain, nil
	}
	c := d.fieldCipher()
	if c == nil {
		return "", ErrFieldsLocked
	}
	value, err := c.Decrypt(enc)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func sortTagsByName(tags []*Tag) {
	sort.SliceStable(tags, func(i, j int) bool {
		return strings.ToLower(tags[i].Name) < strings.ToLower(tags[j].Name)
	})
}

// tagNameTaken 检查标签名是否已被其他标签使用（区分大小写，与旧的 UNIQUE 约束一致）。
func (d *DB) tagNameTaken(name, exceptID string) (bool, error) {
	tags, err := d.ListTags()
	if err != nil {
		return false, err
	}
	for _, t := range tags {
		if t.ID != exceptID && t.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// EncryptLegacyFields 把仍以明文保存的字段加密（解锁后调用，可重复调用）。
// 返回加密的字段行数。
func (d *DB) EncryptLegacyFields() (int, error) {
	c := d.fieldCipher()
	if c == nil {
		return 0, ErrFieldsLocked
	}
	type job struct {
		table, plainCol, encCol, placeholderExpr string
	}
	jobs := []job{
		{"tags", "name", "name_enc", "id"},
		{"notebooks", "name", "name_enc", "''"},
		{"smart_views", "name", "name_enc", "''"},
		{"smart_views", "filter_json", "filter_enc", "'{}'"},
		{"attachments", "original_name", "original_name_enc", "''"},
	}
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	total := 0
	for _, j := range jobs {
		rows, err := tx.Query(`SELECT id, ` + j.plainCol + ` FROM ` + j.table + ` WHERE ` + j.encCol + ` IS NULL`)
		if err != nil {
			return 0, err
		}
		type item struct{ id, value string }
		var items []item
		for rows.Next() {
			var it item
			if err := rows.Scan(&it.id, &it.value); err != nil {
				rows.Close()
				return 0, err
			}
			items = append(items, it)
		}
		if err := rows.Close(); err != nil {
			return 0, err
		}
		for _, it := range items {
			enc, err := c.Encrypt([]byte(it.value))
			if err != nil {
				return 0, err
			}
			if _, err := tx.Exec(`UPDATE `+j.table+` SET `+j.encCol+` = ?, `+j.plainCol+` = `+j.placeholderExpr+` WHERE id = ?`, enc, it.id); err != nil {
				return 0, err
			}
			total++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return total, nil
}
