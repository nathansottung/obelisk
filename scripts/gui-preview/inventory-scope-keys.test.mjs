import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import path from 'node:path';
import { startPreview } from './server.mjs';

const root = process.env.OBELISK_GUI_SCOPE_KEY_INPUTS;
const adapter = process.env.OBELISK_GUI_TEST_ADAPTER;
if (!root || !adapter) throw Error('Provide explicit disposable scope-key fixtures and a fresh reader');
const cases = JSON.parse(await readFile(path.join(root, 'scope-cases.json'), 'utf8'));
const digest = bytes => createHash('sha256').update(bytes).digest('hex');

async function observe(item) {
  const before = await readFile(item.file);
  assert.equal(digest(before), item.sha256);
  const server = await startPreview(0, { catalog: item.file, adapter });
  const base = 'http://127.0.0.1:' + server.address().port;
  try {
    const response = await fetch(base + '/mode.mjs');
    const mode = JSON.parse((await response.text()).slice(15, -1));
    assert.equal(mode.enabled, true);
    assert.equal(mode.ok, item.ok, item.name);
    const query = await fetch(base + '/catalog-query?text=&hash=');
    const result = await query.json();
    assert.equal(result.ok, item.ok, item.name);
    if (!item.ok) {
      assert.equal(mode.catalog, undefined, 'Failed scope must not bootstrap partial/empty success');
      assert.equal(result.ids, undefined);
      assert.ok(query.status >= 400);
      // The adapter may report the reader's terminal exit after the native
      // refusal. Both states must retain explicit failure, never catalog data.
      assert.match(mode.error, /scope|catalog (reader|load)/i);
    } else {
      assert.equal(query.status, 200);
      assert.deepEqual(result.ids, mode.catalog.files.map(file => file.id));
      if (item.name.startsWith('legacy-')) assert.equal(mode.catalog.inventoryScope, null);
      else assert.equal(mode.catalog.inventoryScope.complete, true);
    }
    assert.equal(digest(await readFile(item.file)), item.sha256);
  } finally {
    await new Promise(resolve => { server.close(resolve); server.closeAllConnections(); });
    const exit = await server.catalogStopped;
    assert.equal(exit.code, item.ok ? 0 : 1);
    await assert.rejects(fetch(base));
  }
}

test('Original and wrapper/detail scope-key failures refuse before successful bootstrap', async () => {
  for (const item of cases.filter(item => !item.ok)) await observe(item);
});

test('Valid escaped keys and genuine legacy absence reopen after a refused scope', async () => {
  // Each launch starts a new native reader. No failed fixture may borrow the
  // previous launch's validated scope or records, or poison the next valid one.
  const good = cases.filter(item => item.ok);
  await observe(good[0]);
  await observe(cases.find(item => !item.ok));
  for (const item of good) await observe(item);
});
