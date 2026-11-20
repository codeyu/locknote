import test from 'node:test';
import assert from 'node:assert/strict';

import { attachmentMarkdown, markdownUrlTransform, parseAttachmentId } from './imageAttachments.js';

test('attachmentMarkdown creates stable NaishoMemo image references', () => {
  assert.equal(
    attachmentMarkdown({ id: 'abc-123', originalName: 'Screenshot 1.png' }),
    '![Screenshot 1.png](naisho-attachment://abc-123)',
  );
});

test('parseAttachmentId only accepts NaishoMemo attachment URLs', () => {
  assert.equal(parseAttachmentId('naisho-attachment://abc-123'), 'abc-123');
  assert.equal(parseAttachmentId('https://example.com/image.png'), null);
  assert.equal(parseAttachmentId(undefined), null);
});

test('markdownUrlTransform preserves NaishoMemo attachment image URLs', () => {
  assert.equal(
    markdownUrlTransform('naisho-attachment://abc-123'),
    'naisho-attachment://abc-123',
  );
});

test('markdownUrlTransform keeps normal safe URLs and strips unsafe protocols', () => {
  assert.equal(markdownUrlTransform('https://example.com/image.png'), 'https://example.com/image.png');
  assert.equal(markdownUrlTransform('/relative/image.png'), '/relative/image.png');
  assert.equal(markdownUrlTransform('javascript:alert(1)'), '');
});
