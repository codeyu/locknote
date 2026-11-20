// 拖入文件导入为笔记相关的纯函数。

import { SEALED_FILE_EXTENSION } from './sealedFile.js';

export const MAX_IMPORT_TEXT_FILE_SIZE = 10 * 1024 * 1024;
export const MAX_IMPORT_SEALED_FILE_SIZE = 40 * 1024 * 1024;

export interface FileLike {
  name: string;
  size: number;
  type: string;
}

export interface ClassifiedFiles<T extends FileLike> {
  text: T[];
  sealed: T[];
  images: T[];
  skipped: T[];
}

const TEXT_FILE_PATTERN = /\.(md|markdown|txt)$/i;

export function isImportableTextName(name: string): boolean {
  return TEXT_FILE_PATTERN.test(name);
}

export function isImageFile(file: FileLike): boolean {
  return file.type.startsWith('image/');
}

/** 把拖入的文件分成文本、.locknote、图片和跳过（不支持、为空或过大）几类。 */
export function classifyDroppedFiles<T extends FileLike>(files: T[]): ClassifiedFiles<T> {
  const result: ClassifiedFiles<T> = { text: [], sealed: [], images: [], skipped: [] };
  for (const file of files) {
    const lower = file.name.toLowerCase();
    if (lower.endsWith(SEALED_FILE_EXTENSION)) {
      if (file.size > 0 && file.size <= MAX_IMPORT_SEALED_FILE_SIZE) result.sealed.push(file);
      else result.skipped.push(file);
    } else if (isImportableTextName(file.name)) {
      if (file.size > 0 && file.size <= MAX_IMPORT_TEXT_FILE_SIZE) result.text.push(file);
      else result.skipped.push(file);
    } else if (isImageFile(file)) {
      result.images.push(file);
    } else {
      result.skipped.push(file);
    }
  }
  return result;
}

/** 文件拖放区域：这些区域自己处理拖入的文件，全局导入不接管。 */
export type FileDropZone = 'editor' | 'images';

/**
 * 判断某个拖放区域是否自己处理这次拖入：
 * - 编辑器：单个图片（插入当前笔记）或单个文本文件（弹窗选择新建或替换）
 * - 图片管理：全部是图片
 * 其余情况交给全局导入（新建笔记 / 打开 .locknote）。
 */
export function zoneHandlesDrop(zone: FileDropZone | null, files: FileLike[]): boolean {
  if (!zone || files.length === 0) return false;
  if (zone === 'images') return files.every(isImageFile);
  if (files.length !== 1) return false;
  const [file] = files;
  return isImageFile(file) || isImportableTextName(file.name);
}

/** 用文件名（去掉扩展名）作为备用标题。 */
export function fallbackTitleFromName(name: string, maxLength: number): string {
  const stem = name.replace(/\.[^.]+$/, '').trim();
  return (stem || name).slice(0, maxLength);
}

/** Uint8Array 转 base64（分块，避免大文件栈溢出）。 */
export function bytesToBase64(bytes: Uint8Array): string {
  let binary = '';
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return btoa(binary);
}
