// Package quickunlock 实现 PIN 快速解锁。
//
// 设计（参考 Bitwarden “重启后需要主密码”模式）：
//   - 用户用主密码解锁后，用 PIN 派生的密钥把 dataKey 包裹起来，只保存在内存里；
//   - 应用锁定时 dataKey 被清除，但包裹后的密文保留，输入 PIN 即可恢复；
//   - 程序退出后内存清空，磁盘上没有任何可以离线猜 PIN 的数据，下次启动必须用主密码；
//   - 连续输错 MaxAttempts 次后丢弃包裹的密钥，只能用主密码解锁。
//
// 本包只依赖标准库，密钥派生函数（Argon2id）由调用方传入。
package quickunlock

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"sync"
)

const (
	// MaxAttempts 是 PIN 连续输错的上限，达到后必须使用主密码。
	MaxAttempts = 5
	// MinPINLength / MaxPINLength 限定 PIN 为 4–12 位数字。
	MinPINLength = 4
	MaxPINLength = 12

	saltSize = 16
	keySize  = 32
)

var (
	ErrInvalidPIN       = fmt.Errorf("PIN 必须是 %d–%d 位数字", MinPINLength, MaxPINLength)
	ErrNotArmed         = errors.New("PIN 解锁不可用，请使用主密码")
	ErrTooManyAttempts  = errors.New("PIN 错误次数过多，请使用主密码解锁")
	ErrInvalidDataKey   = errors.New("dataKey 长度不正确")
	errDerivedKeyLength = errors.New("派生密钥长度不正确")
)

// WrongPINError 表示 PIN 不正确，Remaining 为剩余可尝试次数。
type WrongPINError struct {
	Remaining int
}

func (e *WrongPINError) Error() string {
	return fmt.Sprintf("PIN 不正确，还可以再试 %d 次", e.Remaining)
}

// KDF 从秘密和盐派生 32 字节密钥。
type KDF func(secret string, salt []byte) []byte

// State 保存内存中的 PIN 包裹密钥。零值不可用，请用 New 创建。
type State struct {
	mu       sync.Mutex
	kdf      KDF
	salt     []byte
	wrapped  []byte
	failures int
}

// New 创建一个未启用的 State。
func New(kdf KDF) *State {
	return &State{kdf: kdf}
}

// ValidatePIN 检查 PIN 是否为 4–12 位 ASCII 数字。
func ValidatePIN(pin string) error {
	if len(pin) < MinPINLength || len(pin) > MaxPINLength {
		return ErrInvalidPIN
	}
	for i := 0; i < len(pin); i++ {
		if pin[i] < '0' || pin[i] > '9' {
			return ErrInvalidPIN
		}
	}
	return nil
}

func wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func gcmFor(key []byte) (cipher.AEAD, error) {
	if len(key) != keySize {
		return nil, errDerivedKeyLength
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Arm 用 PIN 包裹 dataKey 并保存在内存中，同时清零失败计数。
func (s *State) Arm(dataKey []byte, pin string) error {
	if err := ValidatePIN(pin); err != nil {
		return err
	}
	if len(dataKey) != keySize {
		return ErrInvalidDataKey
	}
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}
	pinKey := s.kdf(pin, salt)
	defer wipe(pinKey)
	gcm, err := gcmFor(pinKey)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	wrapped := gcm.Seal(nonce, nonce, dataKey, nil)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearLocked()
	s.salt = salt
	s.wrapped = wrapped
	s.failures = 0
	return nil
}

// Armed 表示当前能否用 PIN 解锁。
func (s *State) Armed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wrapped != nil
}

// Unlock 用 PIN 取回 dataKey。输错时返回 *WrongPINError，
// 达到 MaxAttempts 次后丢弃包裹密钥并返回 ErrTooManyAttempts。
func (s *State) Unlock(pin string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.wrapped == nil {
		return nil, ErrNotArmed
	}
	if err := ValidatePIN(pin); err != nil {
		return nil, s.failLocked()
	}
	pinKey := s.kdf(pin, s.salt)
	defer wipe(pinKey)
	gcm, err := gcmFor(pinKey)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(s.wrapped) < nonceSize {
		s.clearLocked()
		return nil, ErrNotArmed
	}
	dataKey, err := gcm.Open(nil, s.wrapped[:nonceSize], s.wrapped[nonceSize:], nil)
	if err != nil {
		return nil, s.failLocked()
	}
	s.failures = 0
	return dataKey, nil
}

func (s *State) failLocked() error {
	s.failures++
	if s.failures >= MaxAttempts {
		s.clearLocked()
		return ErrTooManyAttempts
	}
	return &WrongPINError{Remaining: MaxAttempts - s.failures}
}

// Disarm 丢弃内存中的包裹密钥。
func (s *State) Disarm() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearLocked()
}

func (s *State) clearLocked() {
	wipe(s.wrapped)
	wipe(s.salt)
	s.wrapped = nil
	s.salt = nil
	s.failures = 0
}
