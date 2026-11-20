package core

import (
	"log"

	"locknote/internal/crypto"
)

// vaultFieldCipher 用 dataKey 加解密数据库中的短字段（标签名、笔记本名等）。
type vaultFieldCipher struct {
	crypto *crypto.Service
	key    []byte
}

func (v *vaultFieldCipher) Encrypt(plaintext []byte) ([]byte, error) {
	return v.crypto.Encrypt(v.key, plaintext)
}

func (v *vaultFieldCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	return v.crypto.Decrypt(v.key, ciphertext)
}

// installFieldCipherLocked 在解锁时调用（调用方持有 c.mu）：
// 让数据库层加解密元数据字段，并把旧版本留下的明文字段加密。
func (c *Core) installFieldCipherLocked(dataKey []byte) {
	previous := c.fieldCipher
	c.fieldCipher = &vaultFieldCipher{crypto: c.cryptoService, key: append([]byte(nil), dataKey...)}
	c.db.SetFieldCipher(c.fieldCipher)
	if previous != nil {
		wipe(previous.key)
	}
	if n, err := c.db.EncryptLegacyFields(); err != nil {
		log.Printf("[Fields] encrypt legacy fields failed: %v", err)
	} else if n > 0 {
		log.Printf("[Fields] encrypted %d legacy fields", n)
	}
}

// clearFieldCipherLocked 在锁定时调用（调用方持有 c.mu）。
func (c *Core) clearFieldCipherLocked() {
	c.db.SetFieldCipher(nil)
	if c.fieldCipher != nil {
		wipe(c.fieldCipher.key)
		c.fieldCipher = nil
	}
}
