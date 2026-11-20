import assert from 'node:assert/strict';
import test from 'node:test';
import {
  MAX_IMPORT_TEXT_FILE_SIZE,
  bytesToBase64,
  classifyDroppedFiles,
  fallbackTitleFromName,
  zoneHandlesDrop,
} from './textFileImport.js';

const f = (name: string, size = 10, type = '') => ({ name, size, type });

test('classifyDroppedFiles sorts files by kind and skips bad ones', () => {
  const files = [
    f('a.md'),
    f('b.TXT'),
    f('c.markdown'),
    f('secret.locknote'),
    f('photo.png', 10, 'image/png'),
    f('doc.pdf'),
    f('empty.txt', 0),
    f('huge.md', MAX_IMPORT_TEXT_FILE_SIZE + 1),
  ];
  const result = classifyDroppedFiles(files);
  assert.deepEqual(result.text.map((x) => x.name), ['a.md', 'b.TXT', 'c.markdown']);
  assert.deepEqual(result.sealed.map((x) => x.name), ['secret.locknote']);
  assert.deepEqual(result.images.map((x) => x.name), ['photo.png']);
  assert.deepEqual(result.skipped.map((x) => x.name), ['doc.pdf', 'empty.txt', 'huge.md']);
});

test('zoneHandlesDrop lets the editor keep single text or image drops only', () => {
  assert.equal(zoneHandlesDrop('editor', [f('a.md')]), true);
  assert.equal(zoneHandlesDrop('editor', [f('p.png', 1, 'image/png')]), true);
  assert.equal(zoneHandlesDrop('editor', [f('a.md'), f('b.md')]), false);
  assert.equal(zoneHandlesDrop('editor', [f('x.locknote')]), false);
  assert.equal(zoneHandlesDrop('editor', [f('x.pdf')]), false);
  assert.equal(zoneHandlesDrop('images', [f('p.png', 1, 'image/png'), f('q.jpg', 1, 'image/jpeg')]), true);
  assert.equal(zoneHandlesDrop('images', [f('p.png', 1, 'image/png'), f('a.md')]), false);
  assert.equal(zoneHandlesDrop(null, [f('a.md')]), false);
  assert.equal(zoneHandlesDrop('editor', []), false);
});

test('fallbackTitleFromName strips the extension', () => {
  assert.equal(fallbackTitleFromName('会议记录.md', 80), '会议记录');
  assert.equal(fallbackTitleFromName('archive.tar.txt', 80), 'archive.tar');
  assert.equal(fallbackTitleFromName('.md', 80), '.md');
  assert.equal(fallbackTitleFromName('abcdef.txt', 3), 'abc');
});

test('bytesToBase64 encodes correctly, including large inputs', () => {
  assert.equal(bytesToBase64(new Uint8Array([76, 79, 67, 75])), 'TE9DSw==');
  const big = new Uint8Array(100000).fill(65);
  assert.equal(bytesToBase64(big), btoa('A'.repeat(100000)));
});
