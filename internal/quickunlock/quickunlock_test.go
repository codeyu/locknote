package quickunlock

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
)

// testKDF 是测试用的快速 KDF；生产代码使用 Argon2id。
func testKDF(secret string, salt []byte) []byte {
	sum := sha256.Sum256(append(append([]byte{}, salt...), secret...))
	return sum[:]
}

var dataKey = bytes.Repeat([]byte{9}, 32)

func TestValidatePIN(t *testing.T) {
	for _, ok := range []string{"1234", "000000", "123456789012"} {
		if err := ValidatePIN(ok); err != nil {
			t.Fatalf("%q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "123", "1234567890123", "12a4", "１２３４", "12 34", "-123"} {
		if err := ValidatePIN(bad); !errors.Is(err, ErrInvalidPIN) {
			t.Fatalf("%q should be invalid, got %v", bad, err)
		}
	}
}

func TestArmAndUnlock(t *testing.T) {
	s := New(testKDF)
	if s.Armed() {
		t.Fatal("new state must not be armed")
	}
	if _, err := s.Unlock("1234"); !errors.Is(err, ErrNotArmed) {
		t.Fatalf("unarmed unlock err = %v", err)
	}
	if err := s.Arm(dataKey, "1234"); err != nil {
		t.Fatal(err)
	}
	if !s.Armed() {
		t.Fatal("state should be armed")
	}
	got, err := s.Unlock("1234")
	if err != nil || !bytes.Equal(got, dataKey) {
		t.Fatalf("unlock = %x, %v", got, err)
	}
	// 解锁后仍保持可用，下次锁定还能用 PIN
	if _, err := s.Unlock("1234"); err != nil {
		t.Fatalf("second unlock: %v", err)
	}
}

func TestWrappedKeyDoesNotContainDataKey(t *testing.T) {
	s := New(testKDF)
	if err := s.Arm(dataKey, "1234"); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(s.wrapped, dataKey) {
		t.Fatal("wrapped key must not contain the plaintext data key")
	}
}

func TestWrongPINCountsDownThenDisarms(t *testing.T) {
	s := New(testKDF)
	if err := s.Arm(dataKey, "2468"); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < MaxAttempts; i++ {
		_, err := s.Unlock("0000")
		var wrong *WrongPINError
		if !errors.As(err, &wrong) || wrong.Remaining != MaxAttempts-i {
			t.Fatalf("attempt %d: err = %v", i, err)
		}
	}
	if _, err := s.Unlock("0000"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("last attempt err = %v", err)
	}
	if s.Armed() {
		t.Fatal("state must be disarmed after too many attempts")
	}
	if _, err := s.Unlock("2468"); !errors.Is(err, ErrNotArmed) {
		t.Fatalf("correct PIN after lockout err = %v", err)
	}
}

func TestSuccessResetsFailureCount(t *testing.T) {
	s := New(testKDF)
	if err := s.Arm(dataKey, "1357"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < MaxAttempts-1; i++ {
		_, _ = s.Unlock("9999")
	}
	if _, err := s.Unlock("1357"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < MaxAttempts-1; i++ {
		if _, err := s.Unlock("9999"); errors.Is(err, ErrTooManyAttempts) {
			t.Fatalf("counter was not reset (attempt %d)", i+1)
		}
	}
}

func TestMalformedPINCountsAsFailure(t *testing.T) {
	s := New(testKDF)
	if err := s.Arm(dataKey, "1234"); err != nil {
		t.Fatal(err)
	}
	var wrong *WrongPINError
	if _, err := s.Unlock("abc"); !errors.As(err, &wrong) || wrong.Remaining != MaxAttempts-1 {
		t.Fatalf("malformed PIN err = %v", err)
	}
}

func TestDisarmAndRearm(t *testing.T) {
	s := New(testKDF)
	if err := s.Arm(dataKey, "1234"); err != nil {
		t.Fatal(err)
	}
	s.Disarm()
	if s.Armed() {
		t.Fatal("disarm should clear state")
	}
	if err := s.Arm(dataKey, "5678"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Unlock("1234"); err == nil {
		t.Fatal("old PIN must not work after re-arming")
	}
	if _, err := s.Unlock("5678"); err != nil {
		t.Fatal(err)
	}
}

func TestArmRejectsBadInput(t *testing.T) {
	s := New(testKDF)
	if err := s.Arm(dataKey, "12"); !errors.Is(err, ErrInvalidPIN) {
		t.Fatalf("short PIN err = %v", err)
	}
	if err := s.Arm([]byte("short"), "1234"); !errors.Is(err, ErrInvalidDataKey) {
		t.Fatalf("short data key err = %v", err)
	}
	if s.Armed() {
		t.Fatal("failed arm must not arm the state")
	}
}
