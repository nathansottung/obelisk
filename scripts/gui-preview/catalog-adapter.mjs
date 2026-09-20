import { spawn } from 'node:child_process';
import path from 'node:path';
import { realpath } from 'node:fs/promises';
import { validateCatalog, validateIDs } from './catalog-protocol.mjs';

export async function startCatalog({ catalog, adapter }) {
  if (!path.isAbsolute(catalog) || !path.isAbsolute(adapter)) throw new Error('Absolute catalog and adapter paths required');
  const root = await realpath(path.join(process.env.LOCALAPPDATA, 'ObeliskDev'));
  const parent = await realpath(path.dirname(catalog));
  const relative = path.relative(root, parent);
  if (!relative || relative.startsWith('..') || path.isAbsolute(relative)) throw new Error('Select a task-owned input directory beneath ObeliskDev');
  const child = spawn(adapter, ['--gui-catalog-readonly', catalog], { windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] });
  let pending, dead = false, closing = false, stderr = '', closePromise;
  let mode = { enabled: true, ok: false, error: 'Catalog reader has not loaded' };
  let pieces = [], bytes = 0;
  const stopped = new Promise(resolve => {
    child.once('exit', (code, signal) => resolve({ code, signal }));
    child.once('error', () => resolve({ code: null, signal: 'spawn-error' }));
  });
  const fail = (message = 'Catalog reader unavailable') => {
    dead = true; mode = { enabled: true, ok: false, error: message };
    pieces = []; bytes = 0;
    if (pending) { const p = pending; pending = null; clearTimeout(p.timer); p.reject(new Error(message)); }
  };
  const abort = message => { fail(message); if (child.exitCode === null && child.signalCode === null) child.kill(); };
  const receive = validate => new Promise((resolve, reject) => {
    if (dead || pending || closing) return reject(new Error('Reader unavailable or busy'));
    pending = { resolve, reject, validate, timer: setTimeout(() => abort('Catalog reader timeout'), 5000) };
  });
  const line = raw => {
    if (dead || closing) return;
    if (!pending) { abort('Unexpected reader response'); return; }
    try {
      const value = JSON.parse(raw.toString('utf8'));
      const result = pending.validate(value);
      const p = pending; pending = null; clearTimeout(p.timer); p.resolve(result);
    } catch { abort('Invalid catalog reader response'); }
  };
  // Commit only complete newline-terminated responses. EOF is not a successful
  // final line; cap accumulated bytes before allocation/parsing.
  child.stdout.on('data', chunk => {
    if (dead || closing) return;
    let start = 0;
    while (start < chunk.length && !dead) {
      const newline = chunk.indexOf(10, start), end = newline < 0 ? chunk.length : newline;
      const part = chunk.subarray(start, end); bytes += part.length;
      if (bytes > 16 * 1024 * 1024) { abort('Catalog response too large'); return; }
      pieces.push(part);
      if (newline < 0) break;
      const raw = Buffer.concat(pieces, bytes); pieces = []; bytes = 0; line(raw); start = newline + 1;
    }
  });
  child.stdout.on('end', () => { if (!closing) fail('Catalog reader output ended'); });
  child.stdout.on('error', () => abort('Catalog reader transport failed'));
  child.on('error', () => fail('Catalog reader could not start'));
  child.on('exit', () => fail('Catalog reader exited; relaunch to read a catalog'));
  child.stdin.on('error', () => { if (!closing) abort('Catalog reader transport failed'); });
  child.stderr.on('data', b => { stderr = (stderr + b).slice(0, 4096); });
  try {
    const data = await receive(value => {
      if (!value || value.ok !== true) throw new Error('Catalog load refused');
      return validateCatalog(value.catalog);
    });
    if (!dead) mode = { enabled: true, ok: true, catalog: data };
  } catch { /* Failure has already invalidated the entire mode. */ }
  const close = () => closePromise ??= (async () => {
    closing = true; fail('Catalog reader stopped'); child.stdin.end();
    const timer = setTimeout(() => { if (child.exitCode === null && child.signalCode === null) child.kill(); }, 500);
    const exit = await stopped; clearTimeout(timer);
    return { pid: child.pid, ...exit, stderr };
  })();
  return { get mode() { return mode; }, close, query: async (text, hash) => {
    if (!mode.ok || dead || closing || pending) throw new Error('Reader unavailable or busy');
    const data = mode.catalog;
    const response = receive(value => {
      if (!value || typeof value.ok !== 'boolean') throw new Error('Invalid query envelope');
      if (!value.ok) {
        if (typeof value.error !== 'string') throw new Error('Invalid failure envelope');
        return { ok: false, error: 'Catalog query refused' };
      }
      return { ok: true, ids: validateIDs(value.ids, data) };
    });
    child.stdin.write(JSON.stringify({ text, hash }) + '\n');
    const result = await response;
    if (dead) throw new Error('Catalog reader unavailable');
    return result;
  } };
}
