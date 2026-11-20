import assert from 'node:assert/strict';
import test from 'node:test';
import { baseName, formatSealedAt, isSealedFileName, isWrongVaultError } from './sealedFile.js';

test('isSealedFileName matches the .locknote extension case-insensitively', () => {
  assert.equal(isSealedFileName('note.locknote'), true);
  assert.equal(isSealedFileName('NOTE.LockNote'), true);
  assert.equal(isSealedFileName('note.txt'), false);
  assert.equal(isSealedFileName('locknote'), false);
});

test('isWrongVaultError detects the backend wrong-key message', () => {
  assert.equal(isWrongVaultError('无法解密：该文件不属于当前密码库，或文件已损坏'), true);
  assert.equal(isWrongVaultError(new Error('无法解密：该文件不属于当前密码库，或文件已损坏')), true);
  assert.equal(isWrongVaultError('不是 LockNote 加密文件'), false);
  assert.equal(isWrongVaultError(undefined), false);
});

test('formatSealedAt formats valid timestamps and ignores invalid ones', () => {
  assert.equal(formatSealedAt('2026-11-20T06:00:00Z', 'en-US').length > 0, true);
  assert.equal(formatSealedAt('', 'en-US'), '');
  assert.equal(formatSealedAt('not a date', 'en-US'), '');
});

test('baseName handles Windows and POSIX paths', () => {
  assert.equal(baseName('C:\\Users\\me\\note.locknote'), 'note.locknote');
  assert.equal(baseName('/home/me/note.locknote'), 'note.locknote');
  assert.equal(baseName('note.locknote'), 'note.locknote');
});
