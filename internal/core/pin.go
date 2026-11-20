package core

import (
	"errors"
	"log"
	"time"

	"locknote/internal/quickunlock"
)

// PIN 快速解锁
//
// PIN 用 dataKey 加密后保存在数据库里（没有 dataKey 就读不到，也无法离线猜 PIN）。
// 每次用主密码（或恢复密钥）解锁后，读出 PIN，用它在内存中包裹 dataKey；
// 之后自动锁定时可以用 PIN 解锁。程序退出后内存清空，必须重新输入主密码。

// IsPINEnabled 返回是否已设置 PIN 解锁。
func (c *Core) IsPINEnabled() (bool, error) {
	encrypted, err := c.db.GetEncryptedPIN()
	if err != nil {
		return false, err
	}
	return encrypted != nil, nil
}

// CanUnlockWithPIN 返回当前锁屏能否使用 PIN（本次运行中已用主密码解锁过且未被作废）。
func (c *Core) CanUnlockWithPIN() bool {
	return c.quickUnlock.Armed()
}

// SetPIN 设置或修改 PIN，需在已解锁状态下调用。
func (c *Core) SetPIN(pin string) error {
	if err := quickunlock.ValidatePIN(pin); err != nil {
		return err
	}
	key, err := c.vaultKeyCopy()
	if err != nil {
		return err
	}
	defer wipe(key)

	encrypted, err := c.cryptoService.Encrypt(key, []byte(pin))
	if err != nil {
		return err
	}
	if err := c.quickUnlock.Arm(key, pin); err != nil {
		return err
	}
	if err := c.db.SetEncryptedPIN(encrypted); err != nil {
		c.quickUnlock.Disarm()
		return err
	}
	return nil
}

// DisablePIN 关闭 PIN 解锁，需在已解锁状态下调用。
func (c *Core) DisablePIN() error {
	if !c.IsUnlocked() {
		return errVaultLocked
	}
	if err := c.db.SetEncryptedPIN(nil); err != nil {
		return err
	}
	c.quickUnlock.Disarm()
	return nil
}

// UnlockWithPIN 用 PIN 解锁。PIN 错误时返回带剩余次数的错误，
// 错误次数过多或不可用时返回提示使用主密码的错误。
func (c *Core) UnlockWithPIN(pin string) (bool, error) {
	dataKey, err := c.quickUnlock.Unlock(pin)
	if err != nil {
		return false, err
	}
	ok, err := c.verifyDataKeyWithFile(dataKey)
	if err != nil || !ok {
		wipe(dataKey)
		c.quickUnlock.Disarm()
		return false, quickunlock.ErrNotArmed
	}

	c.mu.Lock()
	c.dataKey = dataKey
	c.isUnlocked = true
	c.noteService.SetMasterKey(dataKey)
	c.todoService.SetMasterKey(dataKey)
	c.attachmentService.SetMasterKey(dataKey)
	c.installFieldCipherLocked(dataKey)
	c.lastActivity = time.Now()
	c.mu.Unlock()

	// startLockTimer 内部会获取锁，必须在释放锁后调用
	c.startLockTimer()
	return true, nil
}

// armPINFromStorage 在主密码解锁后调用：若已设置 PIN，则在内存中准备 PIN 解锁。
// 失败只记录日志，不影响主密码解锁本身。
func (c *Core) armPINFromStorage(dataKey []byte) {
	encrypted, err := c.db.GetEncryptedPIN()
	if err != nil {
		log.Printf("[PIN] read failed: %v", err)
		return
	}
	if encrypted == nil {
		c.quickUnlock.Disarm()
		return
	}
	pin, err := c.cryptoService.Decrypt(dataKey, encrypted)
	if err != nil {
		log.Printf("[PIN] decrypt failed: %v", err)
		c.quickUnlock.Disarm()
		return
	}
	defer wipe(pin)
	if err := c.quickUnlock.Arm(dataKey, string(pin)); err != nil && !errors.Is(err, quickunlock.ErrInvalidPIN) {
		log.Printf("[PIN] arm failed: %v", err)
	}
}
