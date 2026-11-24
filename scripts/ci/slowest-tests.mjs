// Reports on `go test -json` output: each package's result and time, the slowest
// top-level tests, the output of every failed test and subtest, any test binary that
// did not build, and what a test timeout printed (the panic and its "running tests"
// list, and each test that never finished). It only reports; the go test exit status
// decides the step. Also written to the job summary when GITHUB_STEP_SUMMARY is set.
//
// Usage: node scripts/ci/slowest-tests.mjs GO_TEST_JSON_FILE [COUNT]
import fs from 'node:fs';

const [file, countArg] = process.argv.slice(2);
if (!file) throw Error('Usage: node scripts/ci/slowest-tests.mjs GO_TEST_JSON_FILE [COUNT]');
const count = Number(countArg ?? 30);
const events = [];
for (const line of fs.readFileSync(file, 'utf8').split(/\r?\n/)) {
  if (!line.startsWith('{')) continue;
  try { events.push(JSON.parse(line)); } catch { /* a stray line from a crash */ }
}
if (events.length === 0) throw Error(`${file}: no go test events`);

const key = (pkg, test) => pkg + ' ' + test;
const tests = new Map(); // "pkg test" -> { pkg, test, action, elapsed, output[] }, subtests included
const packages = new Map(); // pkg -> { action, elapsed }
const builds = new Map(); // import path -> { failed, output[] }
const timeouts = []; // the lines a timeout printed, from whichever test they came under
let inTimeout = false;
for (const e of events) {
  if (e.Action === 'build-output' || e.Action === 'build-fail') {
    const b = builds.get(e.ImportPath) ?? builds.set(e.ImportPath, { output: [] }).get(e.ImportPath);
    if (e.Action === 'build-output') b.output.push(e.Output);
    else b.failed = true;
    continue;
  }
  if (e.Action === 'output') {
    const o = e.Output ?? '';
    if (o.startsWith('panic: test timed out')) inTimeout = true;
    if (inTimeout) {
      if (o.trim() === '') inTimeout = false;
      else timeouts.push(o.replace(/\n$/, ''));
    }
  }
  if (!e.Test) {
    const p = packages.get(e.Package) ?? packages.set(e.Package, { output: [] }).get(e.Package);
    if (e.Action === 'output') p.output.push(e.Output ?? '');
    if (['pass', 'fail', 'skip'].includes(e.Action)) { p.action = e.Action; p.elapsed = e.Elapsed ?? 0; }
    continue;
  }
  const k = key(e.Package, e.Test);
  const t = tests.get(k) ?? tests.set(k, { pkg: e.Package, test: e.Test, output: [] }).get(k);
  if (e.Action === 'output') t.output.push(e.Output);
  if (['pass', 'fail', 'skip'].includes(e.Action)) { t.action = e.Action; t.elapsed = e.Elapsed ?? 0; }
}

const lines = [];
for (const [pkg, p] of packages) if (p.action) lines.push(`${pkg}: ${p.action} in ${p.elapsed.toFixed(2)}s`);
const all = [...tests.values()];
const top = all.filter(t => !t.test.includes('/') && t.action).sort((a, b) => b.elapsed - a.elapsed);
lines.push('', `Slowest ${Math.min(count, top.length)} of ${top.length} top-level tests:`);
for (const t of top.slice(0, count)) lines.push(`${t.elapsed.toFixed(2).padStart(9)}s  ${t.test}  ${t.action}`);

for (const t of all.filter(t => t.action === 'fail')) lines.push('', `FAIL ${t.test}`, ...t.output.map(o => o.replace(/\n$/, '')));
for (const [path, b] of builds) if (b.failed) lines.push('', `BUILD FAILED ${path}`, ...b.output.map(o => o.replace(/\n$/, '')));
if (timeouts.length) lines.push('', ...timeouts);
for (const t of all.filter(t => !t.action)) lines.push('', `did not finish: ${t.test}`, ...t.output.map(o => o.replace(/\n$/, '')));
// A crash outside any test (a fatal error, TestMain, a race found after the last
// test) is only in the package's own output. go test's closing lines are left out.
const frame = /^(PASS|FAIL|ok\s|FAIL\s|coverage:)/;
for (const [pkg, p] of packages) {
  const why = p.output.map(o => o.replace(/\n$/, '')).filter(o => o.trim() !== '' && !frame.test(o));
  if (p.action === 'fail' && why.length) lines.push('', `${pkg} output:`, ...why);
}

const text = lines.join('\n') + '\n';
process.stdout.write(text);
if (process.env.GITHUB_STEP_SUMMARY) fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, '```\n' + text + '```\n');
