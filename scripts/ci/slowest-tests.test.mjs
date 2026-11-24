// Tests for slowest-tests.mjs. Run: node --test scripts/ci/slowest-tests.test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'slowest-tests.mjs');
const ev = (o) => JSON.stringify({ Package: 'example/pkg', ...o });

function run(lines, ...args) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'slowest-'));
  const file = path.join(dir, 'go-test.jsonl');
  fs.writeFileSync(file, lines.join('\n') + '\n');
  const r = spawnSync(process.execPath, [script, file, ...args], { encoding: 'utf8', env: { ...process.env, GITHUB_STEP_SUMMARY: '' } });
  fs.rmSync(dir, { recursive: true, force: true });
  return r;
}

test('lists top-level tests slowest first, leaving out subtests', () => {
  const r = run([
    ev({ Action: 'pass', Test: 'TestQuick', Elapsed: 0.5 }),
    ev({ Action: 'pass', Test: 'TestSlow', Elapsed: 90 }),
    ev({ Action: 'pass', Test: 'TestSlow/sub', Elapsed: 89 }),
    ev({ Action: 'pass', Test: 'TestMiddle', Elapsed: 12.25 }),
    ev({ Action: 'pass', Elapsed: 102.75 }),
  ], '2');
  assert.equal(r.status, 0, r.stderr);
  const rows = r.stdout.split('\n').filter(l => /^\s*\d+\.\d+s/.test(l));
  assert.deepEqual(rows.map(l => l.trim().split(/\s+/)[1]), ['TestSlow', 'TestMiddle']);
  assert.match(r.stdout, /example\/pkg: pass in 102\.75s/);
});

test('shows the output of a failed test and which tests a timeout stopped', () => {
  const r = run([
    ev({ Action: 'run', Test: 'TestBroken' }),
    ev({ Action: 'output', Test: 'TestBroken', Output: '    broken_test.go:9: want 1, got 2\n' }),
    ev({ Action: 'fail', Test: 'TestBroken', Elapsed: 1 }),
    ev({ Action: 'output', Output: 'panic: test timed out after 30m0s\n' }),
    ev({ Action: 'output', Output: '\trunning tests:\n' }),
    ev({ Action: 'output', Output: '\t\tTestStuck (12m0s)\n' }),
    ev({ Action: 'output', Output: '\n' }),
    ev({ Action: 'fail', Elapsed: 1800 }),
  ]);
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.stdout, /FAIL TestBroken/);
  assert.match(r.stdout, /want 1, got 2/);
  assert.match(r.stdout, /panic: test timed out after 30m0s/);
  assert.match(r.stdout, /TestStuck \(12m0s\)/);
});

test('refuses a file with no go test events', () => {
  const r = run(['not json']);
  assert.notEqual(r.status, 0);
});

test('a timeout inside a running test shows its list and the test that never finished', () => {
  const r = run([
    ev({ Action: 'run', Test: 'TestStuck' }),
    ev({ Action: 'run', Test: 'TestStuck/sub' }),
    ev({ Action: 'output', Test: 'TestStuck/sub', Output: 'panic: test timed out after 3s\n' }),
    ev({ Action: 'output', Test: 'TestStuck/sub', Output: '\trunning tests:\n' }),
    ev({ Action: 'output', Test: 'TestStuck/sub', Output: '\t\tTestStuck/sub (3s)\n' }),
    ev({ Action: 'output', Test: 'TestStuck/sub', Output: '\n' }),
    ev({ Action: 'fail', Elapsed: 3.2 }),
  ]);
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.stdout, /panic: test timed out after 3s/);
  assert.match(r.stdout, /TestStuck\/sub \(3s\)/);
  assert.match(r.stdout, /did not finish: TestStuck\b/);
});

test('shows why a subtest failed and why a test binary did not build', () => {
  const r = run([
    ev({ Action: 'run', Test: 'TestParent' }),
    ev({ Action: 'run', Test: 'TestParent/child' }),
    ev({ Action: 'output', Test: 'TestParent/child', Output: '    p_test.go:7: child-reason-xyz\n' }),
    ev({ Action: 'fail', Test: 'TestParent/child', Elapsed: 0 }),
    ev({ Action: 'fail', Test: 'TestParent', Elapsed: 0 }),
    JSON.stringify({ ImportPath: 'example/b [example/b.test]', Action: 'build-output', Output: 'b_test.go:3:27: undefined: undefinedThing\n' }),
    JSON.stringify({ ImportPath: 'example/b [example/b.test]', Action: 'build-fail' }),
    JSON.stringify({ Package: 'example/b', Action: 'fail', Elapsed: 0, FailedBuild: 'example/b [example/b.test]' }),
  ]);
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.stdout, /child-reason-xyz/);
  assert.match(r.stdout, /undefined: undefinedThing/);
});
