import { useEffect, useRef, useState } from 'react';
import { AlertCircle, FilePlus } from 'lucide-react';
import { useStore } from '../store';
import { formatMessage, useI18n } from '../i18n';
import * as App from '../../wailsjs/go/main/App';
import { decodeTextBytes, detectTextEncoding, getDroppedTextTitle, TEXT_FILE_ENCODINGS, type TextFileEncoding } from '../droppedTextFile';
import {
  bytesToBase64,
  classifyDroppedFiles,
  fallbackTitleFromName,
  zoneHandlesDrop,
  type FileDropZone,
} from '../textFileImport';

const MAX_TITLE_LENGTH = 80;
const PREVIEW_LENGTH = 800;

interface PendingImport {
  id: number;
  name: string;
  bytes: Uint8Array;
  encoding: TextFileEncoding;
  content: string;
  decodeError: boolean;
}

function decodeFile(id: number, name: string, bytes: Uint8Array): PendingImport {
  try {
    const encoding = detectTextEncoding(bytes);
    return { id, name, bytes, encoding, content: decodeTextBytes(bytes, encoding), decodeError: false };
  } catch {
    return { id, name, bytes, encoding: 'gb18030', content: '', decodeError: true };
  }
}

function hasFiles(event: DragEvent): boolean {
  return Array.from(event.dataTransfer?.types ?? []).includes('Files');
}

function dropZoneOf(target: EventTarget | null): FileDropZone | null {
  if (!(target instanceof Element)) return null;
  const zone = target.closest('[data-file-drop-zone]')?.getAttribute('data-file-drop-zone');
  return zone === 'editor' || zone === 'images' ? zone : null;
}

/**
 * 全局拖放导入：把文件拖到窗口任意位置，
 * 文本文件（.md / .markdown / .txt，可多个）预览后新建为笔记，.locknote 文件用查看器打开。
 * 编辑器和图片管理里能自己处理的拖放（单个文本或图片、纯图片）留给它们。
 */
export function FileDropImporter() {
  const { t } = useI18n();
  const { setNotes, setSelectedNoteId, setCurrentView, setSealedFile } = useStore();
  const [items, setItems] = useState<PendingImport[]>([]);
  const [skipped, setSkipped] = useState<string[]>([]);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const nextIdRef = useRef(1);
  const importFilesRef = useRef<(files: File[]) => Promise<void>>(async () => {});

  useEffect(() => {
    const handleDragOver = (event: DragEvent) => {
      if (!hasFiles(event)) return;
      // 文件拖动一律允许放下，避免 WebView 把文件当网页打开
      event.preventDefault();
      if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy';
      if (!dropZoneOf(event.target)) event.stopPropagation();
    };

    const handleDrop = (event: DragEvent) => {
      if (!hasFiles(event)) return;
      const files = Array.from(event.dataTransfer?.files ?? []);
      if (zoneHandlesDrop(dropZoneOf(event.target), files)) return;
      event.preventDefault();
      event.stopPropagation();
      void importFilesRef.current(files);
    };

    window.addEventListener('dragover', handleDragOver, true);
    window.addEventListener('drop', handleDrop, true);
    return () => {
      window.removeEventListener('dragover', handleDragOver, true);
      window.removeEventListener('drop', handleDrop, true);
    };
  }, []);

  const importFiles = async (files: File[]) => {
    if (files.length === 0) return;
    const { text, sealed, images, skipped: skippedFiles } = classifyDroppedFiles(files);
    const skippedNames = [...images, ...skippedFiles].map((file) => file.name);

    // .locknote：一次打开一个（取第一个）
    if (sealed.length > 0 && text.length === 0) {
      try {
        const bytes = new Uint8Array(await sealed[0].arrayBuffer());
        setSealedFile({ kind: 'data', name: sealed[0].name, data: bytesToBase64(bytes) });
      } catch (err) {
        alert(`${t.sealed.openFailed}：${String(err)}`);
      }
      if (sealed.length > 1 || skippedNames.length > 0) {
        alert(formatMessage(t.importDrop.skippedCount, { count: sealed.length - 1 + skippedNames.length }));
      }
      return;
    }
    skippedNames.push(...sealed.map((file) => file.name));

    if (text.length === 0) {
      alert(t.importDrop.nothingToImport);
      return;
    }

    const decoded: PendingImport[] = [];
    for (const file of text) {
      try {
        const bytes = new Uint8Array(await file.arrayBuffer());
        decoded.push(decodeFile(nextIdRef.current++, file.name, bytes));
      } catch {
        skippedNames.push(file.name);
      }
    }
    setError('');
    setSkipped(skippedNames);
    setItems(decoded);
  };

  importFilesRef.current = importFiles;

  const close = () => {
    setItems([]);
    setSkipped([]);
    setError('');
  };

  const changeEncoding = (id: number, encoding: TextFileEncoding) => {
    setItems((current) =>
      current.map((item) => {
        if (item.id !== id) return item;
        try {
          return { ...item, encoding, content: decodeTextBytes(item.bytes, encoding), decodeError: false };
        } catch {
          return { ...item, encoding, content: '', decodeError: true };
        }
      }),
    );
  };

  const importable = items.filter((item) => !item.decodeError);

  const handleConfirm = async () => {
    if (importable.length === 0) return;
    setBusy(true);
    setError('');
    let lastId: string | null = null;
    let created = 0;
    try {
      for (const item of importable) {
        const title = getDroppedTextTitle(item.content, MAX_TITLE_LENGTH) || fallbackTitleFromName(item.name, MAX_TITLE_LENGTH);
        const note = await App.CreateNote(title, item.content);
        lastId = note.id;
        created++;
      }
      close();
    } catch (err) {
      setError(`${formatMessage(t.importDrop.partialFailure, { count: created })}${String(err)}`);
      setItems((current) => current.slice(created));
    } finally {
      try {
        const notesList = await App.ListNotes();
        setNotes(notesList || []);
      } catch {
        // 列表刷新失败不影响已创建的笔记
      }
      if (lastId) {
        setCurrentView('notes');
        setSelectedNoteId(lastId);
      }
      setBusy(false);
    }
  };

  if (items.length === 0) return null;

  const single = items.length === 1 ? items[0] : null;

  return (
    <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/30 p-4" onClick={close}>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="drop-import-title"
        className="flex max-h-[85vh] w-full max-w-lg flex-col rounded-lg border border-gray-200 bg-white p-5 shadow-xl"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-center gap-2">
          <FilePlus className="h-5 w-5 text-accent" />
          <h2 id="drop-import-title" className="text-base font-semibold text-gray-900">
            {single ? t.importDrop.titleSingle : formatMessage(t.importDrop.titleMultiple, { count: items.length })}
          </h2>
        </div>
        <p className="mt-2 text-sm text-gray-600">{t.importDrop.desc}</p>

        <div className="mt-4 min-h-0 flex-1 space-y-3 overflow-y-auto">
          {items.map((item) => (
            <div key={item.id} className={single ? '' : 'rounded border border-gray-200 p-3'}>
              <div className="flex items-center justify-between gap-3">
                <span className="min-w-0 break-all text-sm font-medium text-gray-800">{item.name}</span>
                <select
                  aria-label={t.editor.dropEncoding}
                  value={item.encoding}
                  onChange={(event) => changeEncoding(item.id, event.target.value as TextFileEncoding)}
                  className="shrink-0 rounded border border-gray-200 bg-white px-2 py-1 text-xs text-gray-700 focus:outline-none focus:ring-2 focus:ring-accent/50"
                >
                  {TEXT_FILE_ENCODINGS.map(({ value, label }) => (
                    <option key={value} value={value}>
                      {label}
                    </option>
                  ))}
                </select>
              </div>
              <pre
                className={`mt-2 overflow-auto whitespace-pre-wrap break-words rounded border border-gray-200 bg-gray-50 p-2 text-xs text-gray-700 ${
                  single ? 'max-h-48 min-h-20' : 'max-h-16'
                }`}
              >
                {item.decodeError
                  ? t.editor.dropDecodeError
                  : item.content.slice(0, single ? PREVIEW_LENGTH : 160)}
              </pre>
            </div>
          ))}
        </div>

        {skipped.length > 0 && (
          <p className="mt-3 text-xs text-gray-500">
            {formatMessage(t.importDrop.skippedCount, { count: skipped.length })}：{skipped.join('、')}
          </p>
        )}
        {error && (
          <div className="mt-3 flex items-center gap-2 text-sm text-red-500">
            <AlertCircle className="h-4 w-4 shrink-0" />
            {error}
          </div>
        )}

        <div className="mt-4 flex justify-end gap-2">
          <button
            type="button"
            className="rounded border border-gray-200 px-3 py-2 text-sm hover:bg-gray-50"
            onClick={close}
          >
            {t.common.cancel}
          </button>
          <button
            type="button"
            autoFocus
            className="rounded bg-accent px-3 py-2 text-sm text-white hover:bg-primary-600 disabled:cursor-not-allowed disabled:opacity-50"
            disabled={busy || importable.length === 0}
            onClick={() => void handleConfirm()}
          >
            {busy
              ? t.common.loading
              : importable.length > 1
                ? formatMessage(t.importDrop.createMultiple, { count: importable.length })
                : t.importDrop.createSingle}
          </button>
        </div>
      </div>
    </div>
  );
}
