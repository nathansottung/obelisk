import { spawn } from 'node:child_process';
import path from 'node:path';
import { realpath } from 'node:fs/promises';
import { validateCatalog, validateIDs, validateSnapshots, validateMatches } from './catalog-protocol.mjs';
import { randomBytes } from 'node:crypto';
import { decodeCatalogResponse } from './catalog-names.mjs';

// The reader reports the producing binary's version on its first response. It is
// display-only provenance: absent on older readers, and never used for decisions.
const readerVersion = value => {
  if (value === undefined) return undefined;
  if (typeof value !== 'string' || !/^[0-9A-Za-z][0-9A-Za-z.+_-]{0,63}$/.test(value)) throw new Error('Invalid reader version');
  return value;
};

// A well-formed {"ok":false,"error":...} load response: the reader's own refusal,
// after which it exits by itself.
class ReaderRefusal extends Error {}

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
  const alive = () => child.exitCode === null && child.signalCode === null;
  const abort = message => { fail(message); if (alive()) child.kill(); };
  // After a refusal the reader has answered and is exiting with its own status.
  // Killing it now would race that exit and record a signal instead, so the mode
  // fails at once and the reader gets a bounded grace period to exit.
  const refused = message => {
    fail(message);
    const timer = setTimeout(() => { if (alive()) child.kill(); }, 2000);
    stopped.then(() => clearTimeout(timer));
  };
  const receive = validate => new Promise((resolve, reject) => {
    if (dead || pending || closing) return reject(new Error('Reader unavailable or busy'));
    pending = { resolve, reject, validate, timer: setTimeout(() => abort('Catalog reader timeout'), 5000) };
  });
  const line = raw => {
    if (dead || closing) return;
    if (!pending) { abort('Unexpected reader response'); return; }
    try {
      const value = decodeCatalogResponse(raw);
      const result = pending.validate(value);
      const p = pending; pending = null; clearTimeout(p.timer); p.resolve(result);
    } catch (error) {
      if (error instanceof ReaderRefusal) refused(error.message);
      else abort('Invalid catalog reader response');
    }
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
    const loaded = await receive(value => {
      if (value && value.ok === false && typeof value.error === 'string') throw new ReaderRefusal('Catalog load refused');
      if (!value || value.ok !== true) throw new Error('Catalog load refused');
      return { version: readerVersion(value.version), catalog: validateCatalog(value.catalog) };
    });
    if (!dead) mode = { enabled: true, ok: true, ...(loaded.version ? { readerVersion: loaded.version } : {}), catalog: loaded.catalog };
  } catch { /* Failure has already invalidated the entire mode. */ }
  const close = () => closePromise ??= (async () => {
    closing = true; fail('Catalog reader stopped'); child.stdin.end();
    const timer = setTimeout(() => { if (child.exitCode === null && child.signalCode === null) child.kill(); }, 500);
    const exit = await stopped; clearTimeout(timer);
    return { pid: child.pid, ...exit, stderr };
  })();
  return { get mode() { return mode; }, close, query: async (text, hash, exact, enumerate = false) => {
    if (!mode.ok || dead || closing || pending) throw new Error('Reader unavailable or busy');
    const data = mode.catalog;
    const response = receive(value => {
      if (!value || typeof value.ok !== 'boolean') throw new Error('Invalid query envelope');
      if (!value.ok) {
        if (typeof value.error !== 'string') throw new Error('Invalid failure envelope');
        return { ok: false, error: 'Catalog query refused' };
      }
      const ids=validateIDs(value.ids, data);
      if(enumerate && (value.complete!==true || value.digest!==data.digest || value.count!==data.files.length || ids.length!==data.files.length))throw Error('Incomplete recorded enumeration');
      return { ok: true, ids };
    });
    child.stdin.write(JSON.stringify(enumerate ? {enumerate:true} : { text, hash, ...(exact === undefined ? {} : { exact }) }) + '\n');
    const result = await response;
    if (dead) throw new Error('Catalog reader unavailable');
    return result;
  } };
}

export async function startCatalogSession({ catalogs, adapter }) {
  if (!Array.isArray(catalogs) || catalogs.length !== 2) throw new Error('Exactly two explicit catalogs required');
  const readers = [];
  let snapshots, failure = '', busy = false, closing;
  const close = () => closing ??= Promise.all(readers.map(r => r.close()));
  try {
    // Sequential startup limits outstanding adoption work; no successful partial session.
    for (const catalog of catalogs) {
      const reader = await startCatalog({ catalog, adapter }); readers.push(reader);
      if (!reader.mode.ok) throw new Error(`Selected snapshot ${readers.length} could not be loaded`);
    }
    if (readers[0].mode.catalog.digest === readers[1].mode.catalog.digest) throw new Error('Duplicate snapshot artifact refused (identical catalog bytes)');
    const files = readers.reduce((n,r) => n+r.mode.catalog.files.length,0);
    const copies = readers.reduce((n,r) => n+r.mode.catalog.files.reduce((m,f) => m+f.copies.length,0),0);
    if (files > 1000 || copies > 1000) throw new Error('Two-snapshot aggregate limit exceeded: 1000 records / 1000 copy occurrences');
    snapshots = validateSnapshots(readers.map((r, i) => ({ handle: randomBytes(16).toString('hex'), label: path.basename(catalogs[i]), catalog: r.mode.catalog })));
    if (Buffer.byteLength(JSON.stringify(snapshots)) > 32 * 1024 * 1024) throw new Error('Session projection too large');
  } catch (error) { failure = error.message; await close(); }
  const mode = () => {
    if (!failure && readers.some(r => !r.mode.ok)) { failure = 'A snapshot reader is unavailable; relaunch the pair'; void close(); }
    const versions = [...new Set(readers.map(r => r.mode.readerVersion).filter(Boolean))];
    return failure ? { enabled: true, multi: true, ok: false, error: failure } : { enabled: true, multi: true, ok: true, ...(versions.length ? { readerVersion: versions.join(' / ') } : {}), snapshots };
  };
  return {
    get mode() { return mode(); }, close,
    async enumerate() {
      if(!mode().ok || closing || busy)throw Error('Session unavailable or busy');
      busy=true;
      try {
        const results=await Promise.all(readers.map(r=>r.query('','',undefined,true)));
        if(results.some(r=>!r.ok)||!mode().ok)throw Error('Incomplete enumeration');
        return snapshots;
      } catch(error) {failure='Recorded enumeration failed; no complete comparison';await close();throw error;}
      finally {busy=false;}
    },
    async query(text, hash, exact, filter) {
      if (!mode().ok || closing || busy) throw new Error('Session unavailable or busy');
      const selected = filter === 'all' ? snapshots : snapshots.filter(s => s.handle === filter);
      if (!selected.length) throw new Error('Unknown snapshot handle');
      busy = true;
      try {
        const groups = await Promise.all(selected.map(async s => {
          const result = await readers[snapshots.indexOf(s)].query(text, hash, exact);
          if (!result.ok) throw new Error('Snapshot query refused');
          return { snapshot: s.handle, ids: result.ids };
        }));
        if (!mode().ok) throw new Error('Snapshot reader failed');
        const result = { ok: true, groups };
        validateMatches(result, snapshots, filter);
        return result;
      } catch (error) { failure = 'Snapshot query failed; no complete result'; await close(); throw error; }
      finally { busy = false; }
    }
  };
}
