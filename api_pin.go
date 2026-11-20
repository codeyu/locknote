package main

// ============ PIN 快速解锁 ============

// IsPINEnabled 返回是否已设置 PIN 解锁。
func (a *App) IsPINEnabled() (bool, error) {
	c, err := a.coreOrErr()
	if err != nil {
		return false, err
	}
	return c.IsPINEnabled()
}

// CanUnlockWithPIN 返回锁屏时能否用 PIN 解锁。
func (a *App) CanUnlockWithPIN() bool {
	c := a.getCoreIfReady()
	if c == nil {
		return false
	}
	return c.CanUnlockWithPIN()
}

// SetPIN 设置或修改 PIN（需已解锁）。
func (a *App) SetPIN(pin string) error {
	c, err := a.coreOrErr()
	if err != nil {
		return err
	}
	a.UpdateActivity()
	return c.SetPIN(pin)
}

// DisablePIN 关闭 PIN 解锁（需已解锁）。
func (a *App) DisablePIN() error {
	c, err := a.coreOrErr()
	if err != nil {
		return err
	}
	a.UpdateActivity()
	return c.DisablePIN()
}

// UnlockWithPIN 用 PIN 解锁。
func (a *App) UnlockWithPIN(pin string) (bool, error) {
	c, err := a.coreOrErr()
	if err != nil {
		return false, err
	}
	return c.UnlockWithPIN(pin)
}
