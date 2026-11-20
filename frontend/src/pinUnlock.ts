// PIN 快速解锁相关的纯函数，规则需与后端 internal/quickunlock 保持一致。

export const MIN_PIN_LENGTH = 4;
export const MAX_PIN_LENGTH = 12;

export function isValidPIN(pin: string): boolean {
  return new RegExp(`^[0-9]{${MIN_PIN_LENGTH},${MAX_PIN_LENGTH}}$`).test(pin);
}

/** 只保留数字并截断到最大长度，用于输入框。 */
export function sanitizePINInput(value: string): string {
  return value.replace(/[^0-9]/g, '').slice(0, MAX_PIN_LENGTH);
}

export type PinErrorKind =
  | { kind: 'wrong'; remaining: number }
  | { kind: 'usePassword' }
  | { kind: 'other'; message: string };

/**
 * 把后端返回的 PIN 解锁错误归类：
 * - “PIN 不正确，还可以再试 N 次” → wrong
 * - 含“请使用主密码”（次数用尽或不可用）→ usePassword
 */
export function classifyPinError(error: unknown): PinErrorKind {
  const message = error instanceof Error ? error.message : String(error ?? '');
  const wrong = /PIN 不正确，还可以再试 (\d+) 次/.exec(message);
  if (wrong) return { kind: 'wrong', remaining: Number(wrong[1]) };
  if (message.includes('请使用主密码')) return { kind: 'usePassword' };
  return { kind: 'other', message };
}
