// .locknote 加密文件相关的纯函数，便于单元测试。

export const SEALED_FILE_EXTENSION = '.locknote';

export function isSealedFileName(name: string): boolean {
  return name.toLowerCase().endsWith(SEALED_FILE_EXTENSION);
}

/** 后端在“不是当前密码库的密钥”时返回的错误标记，见 internal/sealedfile.ErrWrongKey。 */
const WRONG_VAULT_MARKER = '不属于当前密码库';

/** 判断打开失败是否因为文件来自另一个密码库（可改用恢复密钥再试）。 */
export function isWrongVaultError(error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error ?? '');
  return message.includes(WRONG_VAULT_MARKER);
}

/** 把 RFC3339 时间格式化为本地时间；无效时返回空字符串。 */
export function formatSealedAt(iso: string, locale: string): string {
  if (!iso) return '';
  const time = Date.parse(iso);
  if (Number.isNaN(time)) return '';
  return new Date(time).toLocaleString(locale, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/** 取文件路径中的文件名（兼容 Windows 和 POSIX 分隔符）。 */
export function baseName(path: string): string {
  const parts = path.split(/[\\/]/);
  return parts[parts.length - 1] || path;
}

/** 要在查看器中打开的 .locknote：本地路径（双击、选择文件）或拖入窗口的文件内容（base64）。 */
export type SealedFileSource =
  | { kind: 'path'; path: string }
  | { kind: 'data'; name: string; data: string };
