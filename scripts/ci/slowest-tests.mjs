// Reports on `go test -json` output: each package's result and time, the slowest
// top-level tests, the output of every failed test, and the "running tests" list a
// test timeout prints. It only reports; the go test exit status decides the step.
// Also written to the job summary when GITHUB_STEP_SUMMARY is set.
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

const tests = new Map(); // "pkg test" -> { pkg, test, action, elapsed, output[] }
const packages = new Map(); // pkg -> { action, elapsed, output[] }
for (const e of events) {
  const entry = e.Test
    ? tests.get(e.Package + ' ' + e.Test) ?? tests.set(e.Package + ' ' + e.Test, { pkg: e.Package, test: e.Test, output: [] }).get(e.Package + ' ' + e.Test)
    : packages.get(e.Package) ?? packages.set(e.Package, { output: [] }).get(e.Package);
  if (e.Action === 'output') entry.output.push(e.Output);
  if (['pass', 'fail', 'skip'].includes(e.Action)) { entry.action = e.Action; entry.elapsed = e.Elapsed ?? 0; }
}

const lines = [];
for (const [pkg, p] of packages) if (p.action) lines.push(`${pkg}: ${p.action} in ${p.elapsed.toFixed(2)}s`);
const top = [...tests.values()].filter(t => !t.test.includes('/') && t.action);
top.sort((a, b) => b.elapsed - a.elapsed);
lines.push('', `Slowest ${Math.min(count, top.length)} of ${top.length} top-level tests:`);
for (const t of top.slice(0, count)) lines.push(`${t.elapsed.toFixed(2).padStart(9)}s  ${t.test}  ${t.action}`);

for (const t of top.filter(t => t.action === 'fail')) lines.push('', `FAIL ${t.test}`, ...t.output.map(o => o.replace(/\n$/, '')));
for (const [pkg, p] of packages) {
  const at = p.output.findIndex(o => o.startsWith('panic: test timed out'));
  if (at < 0) continue;
  lines.push('', `${pkg}:`);
  for (const o of p.output.slice(at)) { if (o.trim() === '') break; lines.push(o.replace(/\n$/, '')); }
}

const text = lines.join('\n') + '\n';
process.stdout.write(text);
if (process.env.GITHUB_STEP_SUMMARY) fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, '```\n' + text + '```\n');
