import { useEffect, useState } from 'react';
import { AlertCircle, Check, Copy, FileDown, Lock, X } from 'lucide-react';
import { useStore } from '../store';
import { formatMessage, useI18n } from '../i18n';
import * as App from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';
import { formatSealedAt, isWrongVaultError, type SealedFileSource } from '../sealedFile';

/**
 * 只读查看 .naisho 加密文件。
 * 由 store.sealedFile 驱动：可以是文件路径，也可以是拖入窗口的文件内容。
 * 关闭时清空内容并取下一个待打开的文件。
 * 应用锁定时 MainLayout 卸载、store 重置，解密内容随之丢弃。
 */
function openSource(source: SealedFileSource, recoveryKey: string): Promise<main.SealedFileView> {
  if (source.kind === 'data') {
    return App.OpenSealedFileData(source.name, source.data, recoveryKey);
  }
  return recoveryKey
    ? App.OpenSealedFileWithRecoveryKey(source.path, recoveryKey)
    : App.OpenSealedFile(source.path);
}

export function SealedFileViewer() {
  const { sealedFile, setSealedFile, setNotes } = useStore();
  const { t, language } = useI18n();
  const [view, setView] = useState<main.SealedFileView | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [wrongVault, setWrongVault] = useState(false);
  const [recoveryKey, setRecoveryKey] = useState('');
  const [usedRecoveryKey, setUsedRecoveryKey] = useState('');
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    setView(null);
    setError(null);
    setWrongVault(false);
    setRecoveryKey('');
    setUsedRecoveryKey('');
    setNotice(null);
    setCopied(false);
    if (!sealedFile) return;

    let cancelled = false;
    setBusy(true);
    openSource(sealedFile, '')
      .then((result) => {
        if (!cancelled) setView(result);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(String(err));
        setWrongVault(isWrongVaultError(err));
      })
      .finally(() => {
        if (!cancelled) setBusy(false);
      });
    return () => {
      cancelled = true;
    };
  }, [sealedFile]);

  const close = async () => {
    setView(null);
    setRecoveryKey('');
    setUsedRecoveryKey('');
    let next = '';
    try {
      next = await App.TakePendingSealedFile();
    } catch {
      next = '';
    }
    setSealedFile(next ? { kind: 'path', path: next } : null);
  };

  useEffect(() => {
    if (!sealedFile) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') void close();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [sealedFile]);

  if (!sealedFile) return null;

  const handleOpenWithKey = async () => {
    const key = recoveryKey.trim();
    if (!key) return;
    setBusy(true);
    setError(null);
    try {
      const result = await openSource(sealedFile, key);
      setView(result);
      setUsedRecoveryKey(key);
      setWrongVault(false);
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(false);
      setRecoveryKey('');
    }
  };

  const handleCopy = async () => {
    if (!view) return;
    try {
      await navigator.clipboard.writeText(view.content);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch (err) {
      setError(String(err));
    }
  };

  const handleImport = async () => {
    setBusy(true);
    setError(null);
    try {
      const note =
        sealedFile.kind === 'path'
          ? await App.ImportSealedFile(sealedFile.path, usedRecoveryKey)
          : await App.ImportSealedFileData(sealedFile.data, usedRecoveryKey);
      const notesList = await App.ListNotes();
      setNotes(notesList || []);
      setNotice(formatMessage(t.sealed.imported, { title: note.title }));
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(false);
    }
  };

  const sealedAt = view ? formatSealedAt(view.sealedAt, language) : '';

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30" onClick={() => void close()}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t.sealed.viewerTitle}
        className="w-[min(760px,92vw)] max-h-[85vh] flex flex-col bg-white rounded-xl shadow-xl border border-gray-200"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start gap-3 p-4 border-b border-gray-100">
          <div className="w-10 h-10 bg-primary-100 rounded-lg flex items-center justify-center flex-shrink-0">
            <Lock className="w-5 h-5 text-accent" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="text-xs text-gray-500">{t.sealed.viewerTitle}</div>
            <div className="font-semibold text-gray-900 truncate">{view?.title || view?.fileName || ''}</div>
            {view && (
              <div className="mt-1 text-xs text-gray-500 truncate">
                {view.fileName}
                {view.originalName && ` · ${t.sealed.originalName}：${view.originalName}`}
                {sealedAt && ` · ${t.sealed.sealedAt}：${sealedAt}`}
              </div>
            )}
          </div>
          <button
            onClick={() => void close()}
            className="p-1.5 rounded-lg text-gray-400 hover:bg-gray-100 hover:text-gray-600"
            title={t.sealed.close}
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="flex-1 min-h-0 overflow-y-auto p-4">
          {busy && !view && <div className="text-sm text-gray-500">{t.common.loading}</div>}

          {error && (
            <div className="mb-3 p-3 rounded-lg bg-red-50 border border-red-200 flex items-start gap-2">
              <AlertCircle className="w-4 h-4 text-red-600 flex-shrink-0 mt-0.5" />
              <div className="text-sm text-red-700">
                <div>{t.sealed.openFailed}：{error}</div>
                {wrongVault && <div className="mt-1">{t.sealed.wrongVaultHint}</div>}
              </div>
            </div>
          )}

          {wrongVault && !view && (
            <div className="flex gap-2">
              <input
                type="password"
                value={recoveryKey}
                onChange={(e) => setRecoveryKey(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') void handleOpenWithKey();
                }}
                placeholder={t.sealed.recoveryKeyPlaceholder}
                autoComplete="off"
                className="flex-1 px-3 py-2 border border-gray-200 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-accent/50"
                autoFocus
              />
              <button
                onClick={() => void handleOpenWithKey()}
                disabled={busy || !recoveryKey.trim()}
                className="px-3 py-2 text-sm rounded-lg bg-accent text-white hover:bg-primary-600 disabled:opacity-50"
              >
                {t.sealed.openWithKey}
              </button>
            </div>
          )}

          {view && (
            <pre className="whitespace-pre-wrap break-words font-mono text-sm text-gray-800 select-text">
              {view.content}
            </pre>
          )}
        </div>

        {view && (
          <div className="flex items-center gap-2 p-4 border-t border-gray-100">
            <div className="flex-1 text-xs text-gray-500">
              {notice ? <span className="text-green-700">{notice}</span> : t.sealed.readOnly}
            </div>
            <button
              onClick={() => void handleCopy()}
              className="px-3 py-2 text-sm rounded-lg border border-gray-200 hover:bg-gray-50 flex items-center gap-1.5"
            >
              {copied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
              {copied ? t.sealed.copied : t.sealed.copy}
            </button>
            <button
              onClick={() => void handleImport()}
              disabled={busy || notice !== null}
              className="px-3 py-2 text-sm rounded-lg bg-accent text-white hover:bg-primary-600 disabled:opacity-50 flex items-center gap-1.5"
            >
              <FileDown className="w-4 h-4" />
              {t.sealed.importToVault}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
