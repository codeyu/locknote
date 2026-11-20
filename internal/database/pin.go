package database

// GetEncryptedPIN 返回用 dataKey 加密保存的 PIN；未设置时返回 nil。
func (d *DB) GetEncryptedPIN() ([]byte, error) {
	var encrypted []byte
	err := d.db.QueryRow(`SELECT encrypted_pin FROM settings WHERE id = 1`).Scan(&encrypted)
	if err != nil {
		return nil, err
	}
	if len(encrypted) == 0 {
		return nil, nil
	}
	return encrypted, nil
}

// SetEncryptedPIN 保存用 dataKey 加密的 PIN；传 nil 表示关闭 PIN 解锁。
func (d *DB) SetEncryptedPIN(encrypted []byte) error {
	var value interface{}
	if len(encrypted) > 0 {
		value = encrypted
	}
	_, err := d.db.Exec(`UPDATE settings SET encrypted_pin = ? WHERE id = 1`, value)
	return err
}
