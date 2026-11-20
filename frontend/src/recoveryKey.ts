// 恢复密钥的归一化与校验，规则与 Go 侧 internal/crypto 保持一致：
// 去掉空白与分隔符、转大写，并把易混淆字符按 Crockford 规则映射（I/L→1，O→0）。
// 恢复密钥固定 16 位，字符集为 Crockford Base32（不含 I、L、O、U）。

const RECOVERY_KEY_LENGTH = 16;
const RECOVERY_KEY_ALPHABET = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';

export function normalizeRecoveryKey(input: string): string {
  const stripped = input.replace(/[\s_-]/g, '').toUpperCase();
  return stripped.replace(/[IL]/g, '1').replace(/O/g, '0');
}

export function isValidRecoveryKey(input: string): boolean {
  const normalized = normalizeRecoveryKey(input);
  if (normalized.length !== RECOVERY_KEY_LENGTH) {
    return false;
  }
  for (const char of normalized) {
    if (!RECOVERY_KEY_ALPHABET.includes(char)) {
      return false;
    }
  }
  return true;
}

export type RecoveryKeyWeakness = 'fewDistinct' | 'repeatRun' | 'sequenceRun';

/**
 * 检查恢复密钥是否有明显规律（规则与 Go 侧 internal/keypolicy 一致）：
 * 不同字符少于 8 个、同一字符连续 4 次以上、字符表中连续 5 个以上（如 12345、ZYXWV）。
 * 返回 null 表示没有问题。传入未归一化的输入即可。
 */
export function recoveryKeyWeakness(input: string): RecoveryKeyWeakness | null {
  const key = normalizeRecoveryKey(input);
  if (new Set(key).size < 8) return 'fewDistinct';

  let run = 1;
  for (let i = 1; i < key.length; i++) {
    run = key[i] === key[i - 1] ? run + 1 : 1;
    if (run > 3) return 'repeatRun';
  }

  let up = 1;
  let down = 1;
  for (let i = 1; i < key.length; i++) {
    const diff = RECOVERY_KEY_ALPHABET.indexOf(key[i]) - RECOVERY_KEY_ALPHABET.indexOf(key[i - 1]);
    up = diff === 1 ? up + 1 : 1;
    down = diff === -1 ? down + 1 : 1;
    if (up > 4 || down > 4) return 'sequenceRun';
  }
  return null;
}
