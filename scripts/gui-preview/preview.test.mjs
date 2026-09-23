import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { readFile } from 'node:fs/promises';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { fileURLToPath } from 'node:url';
import { startPreview } from './server.mjs';
import { searchItems, items, activity, operationLabel } from './fixtures.mjs';

test('Search covers name, path, synthetic hash, version, occurrence and medium', () => {
  for (const query of ['Field notes', '/synthetic/package-00/', 'SYNTHETIC-HASH-NOTES-V1', 'v1', 'SIM-OCC-03', 'SIM-TAPE-B']) {
    assert.ok(searchItems({ query }).some(i => i.id === 'SIM-CONTENT-002'), query);
  }
  assert.equal(searchItems({ query: 'nonexistent' }).length, 0);
  assert.equal(searchItems({ query: '雪' })[0].name, 'Scene <draft> & "雪".png');
});
test('Filters combine on the same occurrence and preserve intentional copies', () => {
  assert.deepEqual(searchItems({ medium: 'SIM-TAPE-B', availability: 'online' }), []);
  assert.equal(searchItems({ medium: 'SIM-TAPE-B', project: 'Sample expedition' }).length, 2);
  assert.equal(items[0].occurrences.length, 2);
  assert.notEqual(items[0].hash, items[1].hash);
  assert.equal(searchItems({ availability: 'offline' }).length, 3);
});
test('Unknown, empty, loading and error never become claims of a complete inventory', () => {
  for (const scenario of ['empty', 'loading', 'error']) assert.deepEqual(searchItems({ scenario }), []);
  assert.equal(searchItems({ scenario: 'partial' }).length, 3);
  assert.equal(items[2].occurrences[0].verified, null);
  assert.match(items[2].occurrences[0].scope, /unknown/);
});
test('Failure precedes recording failure; completed and recording recovery remain distinct', () => {
  assert.match(operationLabel(activity[2]), /^FAILED · Unrecorded/);
  assert.match(operationLabel(activity[3]), /^COMPLETED · Unrecorded.*Not verified/);
  assert.match(operationLabel(activity[4]), /^COMPLETED · Recorded.*Not verified/);
  assert.match(activity[4].history, /historical only/);
});
test('Fixture/browser modules have no I/O adapter or unsafe HTML insertion', async () => {
  for (const file of ['fixtures.mjs', 'app.mjs']) {
    const source = await readFile(new URL(file, import.meta.url), 'utf8');
    assert.doesNotMatch(source, /\b(fetch|XMLHttpRequest|WebSocket|EventSource|localStorage|sessionStorage|indexedDB|sendBeacon|innerHTML|outerHTML|insertAdjacentHTML|eval)\b/);
    assert.doesNotMatch(source, /node:|\/api\//);
    const imports = [...source.matchAll(/\bimport\b[^;]+;/g)].map(m => m[0]);
    assert.deepEqual(imports, file === 'fixtures.mjs' ? [] : ["import { scope, media, items, activity, searchItems, operationLabel } from './fixtures.mjs';", "import { catalogMode, renderCatalog } from './catalog-ui.mjs';"]);
  }
});
test('Actual server binds loopback, serves only fixed assets and rejects API/device/mutation routes', async () => {
  const server = await startPreview();
  try {
    assert.equal(server.address().address, '127.0.0.1');
    const base = `http://127.0.0.1:${server.address().port}`;
    for (const path of ['/', '/app.mjs', '/fixtures.mjs', '/style.css']) {
      const res = await fetch(base + path);
      assert.equal(res.status, 200);
      assert.match(res.headers.get('content-security-policy'), /connect-src 'none'/);
      assert.match(res.headers.get('content-security-policy'), /form-action 'none'/);
      await res.text();
    }
    for (const path of ['/api/jobs', '/api/devices', '/api/restore', '/config.json', '/docs/Obelisk.fig', '/docs/OBELISK_Readable_Design_References.zip', '/Untitled.pdf', '/source/Untitled.pdf', '/references/01-library-teal-2x.png', '/before.json', '/..%2fmain.go', '/server.mjs', '/?path=C:/']) {
      const res = await fetch(base + path); assert.equal(res.status, 404, path); await res.text();
    }
    for (const method of ['POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS']) {
      for (const path of ['/', '/api/backup', '/api/restore', '/api/format', '/api/mount', '/api/keys']) {
        const res = await fetch(base + path, { method }); assert.equal(res.status, 405); await res.text();
      }
    }
    const status = await new Promise((resolve, reject) => {
      const req = http.get(base, { headers: { Host: 'foreign.example' } }, res => { res.resume(); resolve(res.statusCode); }); req.on('error', reject);
    });
    assert.equal(status, 403);
  } finally {
    await new Promise(resolve => { server.close(resolve); server.closeAllConnections(); });
    assert.equal(server.listening, false);
  }
});
test('Documented CLI launches on an assigned port, refuses an occupied port, and is stopped/waited', async t => {
  const entry = fileURLToPath(new URL('server.mjs', import.meta.url));
  const child = spawn(process.execPath, [entry, '0'], { windowsHide: true });
  const stopped = once(child, 'exit');
  const timer = setTimeout(() => child.kill(), 10000);
  try {
    const url = await new Promise((resolve, reject) => {
      let output = '';
      child.once('error', reject);
      child.once('exit', () => reject(new Error('CLI exited before readiness')));
      child.stdout.on('data', chunk => { output += chunk; const m = output.match(/http:\/\/127\.0\.0\.1:\d+\//); if (m) resolve(m[0]); });
    });
    const res = await fetch(url);
    assert.equal(res.status, 200); assert.match(await res.text(), /Demo data — no files are read or written/);
    t.diagnostic(`CLI PID ${child.pid}; served ${url}`);
    const conflict = spawn(process.execPath, [entry, new URL(url).port], { windowsHide: true });
    const conflictTimer = setTimeout(() => conflict.kill(), 5000);
    let stderr = ''; conflict.stderr.on('data', chunk => { stderr += chunk; });
    const [code] = await once(conflict, 'exit'); clearTimeout(conflictTimer);
    assert.equal(code, 1); assert.match(stderr, /EADDRINUSE/);
  } finally {
    child.kill(); const [code, signal] = await stopped; clearTimeout(timer);
    assert.ok(code !== null || signal !== null);
    t.diagnostic(`CLI stopped and waited: exit=${code}; signal=${signal}. Forced validation stop; not a manual Ctrl+C check.`);
  }
});
