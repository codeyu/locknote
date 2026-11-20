import assert from 'node:assert/strict';
import test from 'node:test';
import { classifyPinError, isValidPIN, sanitizePINInput } from './pinUnlock.js';

test('isValidPIN accepts 4 to 12 digits only', () => {
  assert.equal(isValidPIN('1234'), true);
  assert.equal(isValidPIN('123456789012'), true);
  assert.equal(isValidPIN('123'), false);
  assert.equal(isValidPIN('1234567890123'), false);
  assert.equal(isValidPIN('12a4'), false);
  assert.equal(isValidPIN(''), false);
});

test('sanitizePINInput keeps digits and truncates', () => {
  assert.equal(sanitizePINInput('12-34 ab'), '1234');
  assert.equal(sanitizePINInput('12345678901234'), '123456789012');
});

test('classifyPinError maps backend messages', () => {
  assert.deepEqual(classifyPinError('PIN 不正确，还可以再试 3 次'), { kind: 'wrong', remaining: 3 });
  assert.deepEqual(classifyPinError(new Error('PIN 错误次数过多，请使用主密码解锁')), { kind: 'usePassword' });
  assert.deepEqual(classifyPinError('PIN 解锁不可用，请使用主密码'), { kind: 'usePassword' });
  assert.deepEqual(classifyPinError('boom'), { kind: 'other', message: 'boom' });
});
