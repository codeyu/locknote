import { useEffect, useState } from 'react';
import { AlertCircle, Check, Hash } from 'lucide-react';
import { useI18n } from '../i18n';
import * as App from '../../wailsjs/go/main/App';
import { MAX_PIN_LENGTH, isValidPIN, sanitizePINInput } from '../pinUnlock';

const inputClassName =
  'w-full rounded-xl border border-gray-200 bg-white px-4 py-3 text-sm text-gray-800 shadow-sm tracking-[0.3em] font-mono transition-all placeholder:tracking-normal placeholder:font-sans placeholder:text-gray-400 hover:border-primary-300 focus:border-accent focus:outline-none focus:ring-4 focus:ring-primary-100';

/** 设置页中的“PIN 快速解锁”卡片：开启、修改、关闭。 */
export function PinSettingsCard() {
  const { t } = useI18n();
  const [enabled, setEnabled] = useState(false);
  const [showDialog, setShowDialog] = useState(false);
  const [pin, setPin] = useState('');
  const [confirmPin, setConfirmPin] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const [busy, setBusy] = useState(false);

  const refresh = async () => {
    try {
      setEnabled(await App.IsPINEnabled());
    } catch {
      setEnabled(false);
    }
  };

  useEffect(() => {
    void refresh();
  }, []);

  const closeDialog = () => {
    setShowDialog(false);
    setPin('');
    setConfirmPin('');
    setError('');
  };

  const handleSave = async () => {
    setError('');
    if (!isValidPIN(pin)) {
      setError(t.pin.invalid);
      return;
    }
    if (pin !== confirmPin) {
      setError(t.pin.mismatch);
      return;
    }
    setBusy(true);
    try {
      await App.SetPIN(pin);
      closeDialog();
      setNotice({ type: 'success', text: t.pin.saved });
      await refresh();
    } catch (err) {
      setError(`${t.pin.saveFailed}：${String(err)}`);
    } finally {
      setBusy(false);
    }
  };

  const handleDisable = async () => {
    setBusy(true);
    setNotice(null);
    try {
      await App.DisablePIN();
      setNotice({ type: 'success', text: t.pin.removed });
      await refresh();
    } catch (err) {
      setNotice({ type: 'error', text: String(err) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="p-6 border border-gray-200 rounded-xl">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-start gap-3">
          <Hash className="w-5 h-5 text-accent mt-0.5" />
          <div>
            <div className="flex items-center gap-2">
              <h3 className="font-semibold text-gray-800">{t.pin.title}</h3>
              <span
                className={`rounded-full px-2 py-0.5 text-xs ${
                  enabled ? 'bg-primary-100 text-accent' : 'bg-gray-100 text-gray-500'
                }`}
              >
                {enabled ? t.pin.enabled : t.pin.disabled}
              </span>
            </div>
            <p className="mt-1 max-w-md text-sm text-gray-500">{t.pin.desc}</p>
          </div>
        </div>
        <div className="flex gap-2 self-start sm:self-center">
          {enabled && (
            <button
              onClick={() => void handleDisable()}
              disabled={busy}
              className="rounded-xl border border-gray-200 px-4 py-2.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50"
            >
              {t.pin.disable}
            </button>
          )}
          <button
            onClick={() => {
              setNotice(null);
              setShowDialog(true);
            }}
            disabled={busy}
            className="rounded-xl bg-accent px-4 py-2.5 text-sm font-medium text-white shadow-sm transition-colors hover:bg-primary-600 disabled:opacity-50"
          >
            {enabled ? t.pin.change : t.pin.enable}
          </button>
        </div>
      </div>

      {notice && (
        <div
          className={`mt-4 flex items-center gap-2 text-sm ${
            notice.type === 'success' ? 'text-green-700' : 'text-red-600'
          }`}
        >
          {notice.type === 'success' ? <Check className="w-4 h-4" /> : <AlertCircle className="w-4 h-4" />}
          {notice.text}
        </div>
      )}

      {showDialog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 px-4" onClick={closeDialog}>
          <div
            role="dialog"
            aria-modal="true"
            aria-label={t.pin.title}
            className="w-full max-w-[400px] rounded-xl border border-gray-200 bg-white p-6 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-5 flex items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <Hash className="w-5 h-5 text-accent" />
                <h3 className="font-semibold text-gray-800">{enabled ? t.pin.change : t.pin.enable}</h3>
              </div>
              <button
                type="button"
                onClick={closeDialog}
                className="rounded-lg px-2 py-1 text-sm text-gray-500 transition-colors hover:bg-gray-100"
              >
                {t.common.close}
              </button>
            </div>
            <div className="space-y-4">
              <div>
                <label className="mb-1.5 block text-sm font-medium text-gray-700">{t.pin.newPin}</label>
                <input
                  type="password"
                  inputMode="numeric"
                  autoComplete="off"
                  maxLength={MAX_PIN_LENGTH}
                  value={pin}
                  onChange={(e) => setPin(sanitizePINInput(e.target.value))}
                  className={inputClassName}
                  autoFocus
                />
              </div>
              <div>
                <label className="mb-1.5 block text-sm font-medium text-gray-700">{t.pin.confirmPin}</label>
                <input
                  type="password"
                  inputMode="numeric"
                  autoComplete="off"
                  maxLength={MAX_PIN_LENGTH}
                  value={confirmPin}
                  onChange={(e) => setConfirmPin(sanitizePINInput(e.target.value))}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') void handleSave();
                  }}
                  className={inputClassName}
                />
              </div>
              {error && (
                <div className="flex items-center gap-2 text-sm text-red-500">
                  <AlertCircle className="w-4 h-4" />
                  {error}
                </div>
              )}
              <p className="text-xs text-gray-500">{t.pin.desc}</p>
              <button
                onClick={() => void handleSave()}
                disabled={busy || !pin || !confirmPin}
                className="w-full rounded-xl bg-accent py-3 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {busy ? t.common.loading : t.common.confirm}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
