package core

import (
	"errors"
	"testing"

	"naishomemo/internal/quickunlock"
)

func TestPINUnlockLifecycle(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SetupPassword("master-pass", "", "K7MQ2X9P4R3WHT6N"); err != nil {
		t.Fatal(err)
	}
	if enabled, _ := c.IsPINEnabled(); enabled {
		t.Fatal("PIN should be disabled by default")
	}
	if err := c.SetPIN("12"); !errors.Is(err, quickunlock.ErrInvalidPIN) {
		t.Fatalf("short PIN err = %v", err)
	}
	if err := c.SetPIN("2580"); err != nil {
		t.Fatal(err)
	}
	if enabled, _ := c.IsPINEnabled(); !enabled {
		t.Fatal("PIN should be enabled")
	}

	c.Lock()
	if !c.CanUnlockWithPIN() {
		t.Fatal("PIN unlock should be available after locking")
	}
	if ok, err := c.UnlockWithPIN("0000"); ok || err == nil {
		t.Fatalf("wrong PIN = %v, %v", ok, err)
	}
	if ok, err := c.UnlockWithPIN("2580"); !ok || err != nil {
		t.Fatalf("PIN unlock = %v, %v", ok, err)
	}
	if !c.IsUnlocked() {
		t.Fatal("core should be unlocked")
	}
	if _, err := c.Notes().Create("t", "body"); err != nil {
		t.Fatalf("notes should work after PIN unlock: %v", err)
	}
	c.Close()

	// 重新启动：PIN 不可直接使用，必须先用主密码
	c2, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	if c2.CanUnlockWithPIN() {
		t.Fatal("PIN must not be usable after restart")
	}
	if _, err := c2.UnlockWithPIN("2580"); !errors.Is(err, quickunlock.ErrNotArmed) {
		t.Fatalf("PIN after restart err = %v", err)
	}
	if ok, err := c2.Unlock("master-pass"); !ok || err != nil {
		t.Fatalf("master unlock = %v, %v", ok, err)
	}
	c2.Lock()
	if ok, err := c2.UnlockWithPIN("2580"); !ok || err != nil {
		t.Fatalf("PIN unlock after master unlock = %v, %v", ok, err)
	}

	if err := c2.DisablePIN(); err != nil {
		t.Fatal(err)
	}
	c2.Lock()
	if c2.CanUnlockWithPIN() {
		t.Fatal("PIN unlock should be gone after disabling")
	}
}

func TestPINLockoutRequiresMasterPassword(t *testing.T) {
	c := newTestCore(t)
	if _, err := c.SetupPassword("master-pass", "", "K7MQ2X9P4R3WHT6N"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetPIN("1111"); err != nil {
		t.Fatal(err)
	}
	c.Lock()
	var last error
	for i := 0; i < quickunlock.MaxAttempts; i++ {
		_, last = c.UnlockWithPIN("9999")
	}
	if !errors.Is(last, quickunlock.ErrTooManyAttempts) {
		t.Fatalf("last err = %v", last)
	}
	if c.CanUnlockWithPIN() {
		t.Fatal("PIN must be unavailable after lockout")
	}
	if ok, _ := c.Unlock("master-pass"); !ok {
		t.Fatal("master password should still work")
	}
	c.Lock()
	if ok, err := c.UnlockWithPIN("1111"); !ok || err != nil {
		t.Fatalf("PIN should work again after master unlock: %v, %v", ok, err)
	}
}

func TestSetPINRequiresUnlockedVault(t *testing.T) {
	c := newTestCore(t)
	if _, err := c.SetupPassword("master-pass", "", "K7MQ2X9P4R3WHT6N"); err != nil {
		t.Fatal(err)
	}
	c.Lock()
	if err := c.SetPIN("1234"); err == nil {
		t.Fatal("SetPIN should fail while locked")
	}
	if err := c.DisablePIN(); err == nil {
		t.Fatal("DisablePIN should fail while locked")
	}
}
