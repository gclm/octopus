import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const localeDir = path.resolve(__dirname, '../public/locale');

const baseLocale = 'zh_hans';
const localeFiles = ['zh_hans', 'zh_hant', 'en'];

function readLocale(name) {
  const filePath = path.join(localeDir, `${name}.json`);
  return JSON.parse(fs.readFileSync(filePath, 'utf8'));
}

function isObject(value) {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function compareShape(base, target, currentPath, diffs) {
  if (isObject(base) && isObject(target)) {
    for (const key of Object.keys(base)) {
      const nextPath = currentPath ? `${currentPath}.${key}` : key;
      if (!(key in target)) {
        diffs.push(`missing key: ${nextPath}`);
        continue;
      }
      compareShape(base[key], target[key], nextPath, diffs);
    }
    for (const key of Object.keys(target)) {
      if (!(key in base)) {
        const nextPath = currentPath ? `${currentPath}.${key}` : key;
        diffs.push(`unexpected key: ${nextPath}`);
      }
    }
    return;
  }

  if (Array.isArray(base) !== Array.isArray(target)) {
    diffs.push(`type mismatch at ${currentPath}: expected ${Array.isArray(base) ? 'array' : typeof base}, got ${Array.isArray(target) ? 'array' : typeof target}`);
    return;
  }

  if (isObject(base) !== isObject(target)) {
    diffs.push(`type mismatch at ${currentPath}: expected ${isObject(base) ? 'object' : typeof base}, got ${isObject(target) ? 'object' : typeof target}`);
  }
}

const baseMessages = readLocale(baseLocale);
let hasError = false;

for (const locale of localeFiles) {
  if (locale === baseLocale) continue;
  const targetMessages = readLocale(locale);
  const diffs = [];
  compareShape(baseMessages, targetMessages, '', diffs);
  if (diffs.length > 0) {
    hasError = true;
    console.error(`Locale shape mismatch for ${locale}:`);
    for (const diff of diffs) {
      console.error(`  - ${diff}`);
    }
  }
}

if (hasError) {
  process.exit(1);
}

console.log('Locale shapes are consistent.');
