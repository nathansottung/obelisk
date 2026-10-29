// Runs the hand-restore and gpg-pin tests and fails if any of them failed, was
// skipped or did not run. The full suite already runs them; this step exists
// because a missing tool makes them skip, and a skip would otherwise pass
// silently. They are the evidence that a package restores by hand with the
// stock tools on this platform: bsdtar on Windows, GNU tar on Linux.
//
// Usage: node scripts/ci/hand-restore.mjs
import { spawnSync } from 'node:child_process';

const required = [
  'TestHandRestore_FromRESTORE_txt',
  'TestHandRestore_SpannedFromRESTORE_txt',
  'TestBuildChunk_GpgOutputIsPinned',
];
const run = spawnSync('go', ['test', '-count=1', '-json', '-run', '^(' + required.join('|') + ')$', '.'], { encoding: 'utf8', maxBuffer: 64 << 20 });
if (run.error) throw run.error;
const final = new Map();
for (const line of run.stdout.split(/\r?\n/)) {
  if (!line.startsWith('{')) continue;
  const e = JSON.parse(line);
  if (e.Action === 'output' && e.Test) process.stdout.write(e.Output);
  if (e.Test && ['pass', 'fail', 'skip'].includes(e.Action)) final.set(e.Test, e.Action);
}
let bad = 0;
for (const t of required) {
  const a = final.get(t) ?? 'did not run';
  console.log(`${a === 'pass' ? 'PASS' : 'NOT PASSED'} ${t}: ${a}`);
  if (a !== 'pass') bad++;
}
if (run.status !== 0 && bad === 0) {
  console.error(run.stderr);
  bad++;
}
if (bad) {
  console.error(`hand-restore gate failed: ${bad} required test(s) did not pass`);
  process.exit(1);
}
console.log('hand-restore gate passed');
