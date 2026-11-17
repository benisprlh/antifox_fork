#!/usr/bin/env node
// Check src t("key")/t('key') references against en-US.json.
// Report missing translation keys and remaining hardcoded Chinese text.
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const SRC = new URL("../web/src/", import.meta.url).pathname;
const enUS = JSON.parse(readFileSync(join(SRC, "i18n/locales/en-US.json"), "utf8"));

function walk(dir, acc = []) {
  for (const e of readdirSync(dir)) {
    const p = join(dir, e);
    const s = statSync(p);
    if (s.isDirectory()) walk(p, acc);
    else if (/\.(tsx?|ts)$/.test(e)) acc.push(p);
  }
  return acc;
}

function hasKey(obj, dotted) {
  return dotted.split(".").reduce((o, k) => (o && typeof o === "object" ? o[k] : undefined), obj) !== undefined;
}

const files = walk(SRC);
const KEY_RE = /\bt\(\s*["'`]([a-zA-Z0-9_.]+)["'`]/g;
const missing = new Map(); // key -> [files]
let refCount = 0;

for (const f of files) {
  const src = readFileSync(f, "utf8");
  let m;
  while ((m = KEY_RE.exec(src))) {
    const key = m[1];
    if (!key.includes(".")) continue; // Skip unnamespaced keys such as variables.
    refCount++;
    if (!hasKey(enUS, key)) {
      const rel = f.replace(SRC, "");
      if (!missing.has(key)) missing.set(key, new Set());
      missing.get(key).add(rel);
    }
  }
}

console.log(`Scanned ${files.length} files and ${refCount} t() references.`);
if (missing.size === 0) {
  console.log("✓ All t() keys exist in en-US.json.");
} else {
  console.log(`✗ ${missing.size} keys missing from en-US.json:`);
  for (const [k, fs] of missing) console.log(`  ${k}  ← ${[...fs].join(", ")}`);
}

// Check all tracked UTF-8 text, including documentation, comments, and mock data.
// Unicode escapes remain available for legacy input aliases and multilingual tests.
const ROOT = fileURLToPath(new URL("../", import.meta.url));
const CJK = /\p{Script=Han}/u;
const residual = [];
const tracked = execFileSync("git", ["ls-files", "-z"], { cwd: ROOT, encoding: "utf8" }).split("\0").filter(Boolean);
for (const f of tracked) {
  let content;
  try {
    const bytes = readFileSync(join(ROOT, f));
    content = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  } catch (error) {
    if (error.code === "ENOENT" || error instanceof TypeError) continue;
    throw error;
  }
  const n = content.split("\n").filter((line) => CJK.test(line)).length;
  if (n > 0) residual.push([f, n]);
}
residual.sort((a, b) => b[1] - a[1]);
console.log(`\nTracked text files containing Chinese: ${residual.length}`);
for (const [f, n] of residual) console.log(`  ${String(n).padStart(4)}  ${f}`);

process.exit(missing.size === 0 && residual.length === 0 ? 0 : 1);
