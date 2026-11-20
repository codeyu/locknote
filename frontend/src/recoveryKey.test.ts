import assert from 'node:assert/strict';
import test from 'node:test';
import { isValidRecoveryKey, normalizeRecoveryKey, recoveryKeyWeakness } from './recoveryKey.js';

test('normalizeRecoveryKey strips separators and uppercases', () => {
  assert.equal(normalizeRecoveryKey('abcd-efgh-jkmn-pqrs'), 'ABCDEFGHJKMNPQRS');
  assert.equal(normalizeRecoveryKey('  abcd efgh jkmn pqrs '), 'ABCDEFGHJKMNPQRS');
});

test('normalizeRecoveryKey maps ambiguous characters', () => {
  assert.equal(normalizeRecoveryKey('ABCDEFGHJKMNPQIL'), 'ABCDEFGHJKMNPQ11');
  assert.equal(normalizeRecoveryKey('OOOOOOOOOOOOOOOO'), '0000000000000000');
});

test('isValidRecoveryKey accepts 16 Crockford characters', () => {
  assert.equal(isValidRecoveryKey('ABCDEFGHJKMNPQRS'), true);
  assert.equal(isValidRecoveryKey('abcd efgh jkmn pqrs'), true);
});

test('isValidRecoveryKey rejects wrong length or characters', () => {
  assert.equal(isValidRecoveryKey(''), false);
  assert.equal(isValidRecoveryKey('ABCDE'), false);
  assert.equal(isValidRecoveryKey('ABCDEFGHJKMNPQR'), false);
  assert.equal(isValidRecoveryKey('ABCDEFGHJKMNPQRSX'), false);
  assert.equal(isValidRecoveryKey('ABCDEFGHJKMNPQR$'), false);
});

test('recoveryKeyWeakness matches the Go key policy', () => {
  assert.equal(recoveryKeyWeakness('AAAAAAAAAAAAAAAA'), 'fewDistinct');
  assert.equal(recoveryKeyWeakness('ABCDABCDABCDABCD'), 'fewDistinct');
  assert.equal(recoveryKeyWeakness('7K3M9QAAAAP2XW4R'), 'repeatRun');
  assert.equal(recoveryKeyWeakness('0123456789ABCDEF'), 'sequenceRun');
  assert.equal(recoveryKeyWeakness('ABCDEFGHJKMNPQRS'), 'sequenceRun');
  assert.equal(recoveryKeyWeakness('ZYXWV7K3M9Q2P4R8'), 'sequenceRun');
  assert.equal(recoveryKeyWeakness('K7MQ2X9P4R3WHT6N'), null);
  assert.equal(recoveryKeyWeakness('k7mq-2x9p-4r3w-ht6n'), null);
  assert.equal(recoveryKeyWeakness('0NXYVSZQ65TX7SW1'), null);
});
