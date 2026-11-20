import assert from 'node:assert/strict';
import test from 'node:test';
import { createHash } from 'node:crypto';
import {
  MIN_PASSWORD_LENGTH,
  PASSPHRASE_SEPARATOR,
  PASSPHRASE_WORDS,
  PASSPHRASE_WORD_COUNT,
  generatePassphrase,
  passphraseEntropyBits,
  randomIndex,
} from './passphrase.js';

test('word list is the BIP-39 English list', () => {
  assert.equal(PASSPHRASE_WORDS.length, 2048);
  assert.equal(new Set(PASSPHRASE_WORDS).size, 2048);
  assert.equal(PASSPHRASE_WORDS.every((w) => /^[a-z]{3,8}$/.test(w)), true);
  const digest = createHash('sha256').update(PASSPHRASE_WORDS.join('\n') + '\n').digest('hex');
  assert.equal(digest, '2f5eed53a4727b4bf8880d8f3f199efc90e58503646d9ff8eff3a2ed3b24dbda');
});

test('generatePassphrase returns five known words joined by dashes', () => {
  const words = new Set(PASSPHRASE_WORDS);
  for (let i = 0; i < 50; i++) {
    const phrase = generatePassphrase();
    const parts = phrase.split(PASSPHRASE_SEPARATOR);
    assert.equal(parts.length, PASSPHRASE_WORD_COUNT);
    assert.equal(parts.every((p) => words.has(p)), true);
    assert.equal(phrase.length >= MIN_PASSWORD_LENGTH, true);
  }
});

test('generatePassphrase uses the provided random source', () => {
  const fixed = (buffer: Uint32Array) => {
    buffer[0] = 0;
  };
  assert.equal(generatePassphrase(3, fixed), 'abandon-abandon-abandon');
  const last = (buffer: Uint32Array) => {
    buffer[0] = 2047;
  };
  assert.equal(generatePassphrase(2, last), 'zoo-zoo');
});

test('randomIndex rejects values that would bias the result', () => {
  // max = 3: limit = floor(2^32 / 3) * 3 = 4294967295, so 4294967295 must be rejected
  const values = [4294967295, 4294967294, 5];
  let calls = 0;
  const source = (buffer: Uint32Array) => {
    buffer[0] = values[calls++];
  };
  assert.equal(randomIndex(3, source), 4294967294 % 3);
  assert.equal(calls, 2);
});

test('randomIndex validates its bound', () => {
  assert.throws(() => randomIndex(0));
  assert.throws(() => randomIndex(1.5));
});

test('five words give 55 bits and the minimum length is 8', () => {
  assert.equal(passphraseEntropyBits(), 55);
  assert.equal(MIN_PASSWORD_LENGTH, 8);
});
