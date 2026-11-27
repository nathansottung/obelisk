// Tests for go-known-failures.mjs. Run: node --test scripts/ci/go-known-failures.test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'go-known-failures.mjs');
const ev = (o) => JSON.stringify({ Package: 'example/pkg', ...o });

function run(lines) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'known-failures-'));
  const file = path.join(dir, 'go-test.jsonl');
  fs.writeFileSync(file, lines.join('\n') + '\n');
  const r = spawnSync(process.execPath, [script, file], { encoding: 'utf8' });
  fs.rmSync(dir, { recursive: true, force: true });
  return r;
}

// The must-pass names, as the gate prints them.
function mustPassNames() {
  const r = run([ev({ Action: 'pass', Test: 'TestSomethingElse' }), ev({ Action: 'pass' })]);
  return [...r.stdout.matchAll(/MUST PASS [^:]*: (\S+)/g)].map(m => m[1]);
}

// Every must-pass test passes, with the given lines added.
function allPassing(...extra) {
  return [...mustPassNames().map(t => ev({ Action: 'pass', Test: t })), ...extra, ev({ Action: 'pass' })];
}

test('passes when every must-pass test passes', () => {
  const r = run(allPassing());
  assert.equal(r.status, 0, r.stdout + r.stderr);
});

test('the key page flow and the hand restores are must-pass', () => {
  const names = mustPassNames();
  for (const t of ['TestUI_FirstStartToEncryptedBuildThroughThePage', 'TestHandRestore_OldPerPackageKeyAndNewSharedKey']) {
    assert.ok(names.includes(t), `${t} must be on the must-pass list`);
  }
});

test('a skipped must-pass test fails the gate', () => {
  const names = mustPassNames();
  for (const t of ['TestUI_FirstStartToEncryptedBuildThroughThePage', 'TestHandRestore_OldPerPackageKeyAndNewSharedKey']) {
    const lines = allPassing().filter(l => !l.includes(`"Test":"${t}"`));
    const r = run([...lines, ev({ Action: 'skip', Test: t })]);
    assert.notEqual(r.status, 0, `a skip of ${t} passed the gate:\n${r.stdout}`);
    assert.match(r.stdout, new RegExp(`NOT PASSED \\(skip\\): ${t}`));
  }
  assert.ok(names.length > 0);
});

test('a must-pass test that did not run fails the gate', () => {
  const t = 'TestHandRestore_OldPerPackageKeyAndNewSharedKey';
  const r = run(allPassing().filter(l => !l.includes(`"Test":"${t}"`)));
  assert.notEqual(r.status, 0, r.stdout);
  assert.match(r.stdout, new RegExp(`NOT PASSED \\(did not run\\): ${t}`));
});

test('a must-pass test whose subtest skipped fails the gate', () => {
  const t = 'TestHandRestore_OldPerPackageKeyAndNewSharedKey';
  const r = run(allPassing(ev({ Action: 'skip', Test: t + '/old_package_with_its_own_key_(frozen)' })));
  assert.notEqual(r.status, 0, `a skipped subtest of ${t} passed the gate:\n${r.stdout}`);
  assert.match(r.stdout, new RegExp(`NOT PASSED \\(a subtest skipped\\): ${t}`));
});

test('a skipped subtest of a test that is not must-pass is only listed', () => {
  const r = run(allPassing(ev({ Action: 'pass', Test: 'TestOther' }), ev({ Action: 'skip', Test: 'TestOther/sub' })));
  assert.equal(r.status, 0, r.stdout + r.stderr);
});
