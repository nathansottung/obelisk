// CI gate for `go test -json` output on Windows. Passes only when the set of failing
// top-level tests is exactly the known list below: a new failure fails the gate,
// and so does a known failure that passed, was skipped or did not run. A package
// failure with no failing test (build error, crash outside a test) also fails.
// Skips are listed; they are not passes.
//
// Usage: node scripts/ci/go-known-failures.mjs GO_TEST_JSON_FILE
import fs from 'node:fs';

// Tar member-name handling with non-ASCII and hostile names under the Windows
// system tar (bsdtar). Tracked as a known Windows gap; remove entries as it is fixed.
const known = [
  'TestBuildChunk_UnicodeFilenames_ExactMembers',
  'TestBuildChunk_UnicodeSourceRoot',
  'TestBuildChunk_UnicodeStagingDir',
  'TestBuildChunk_WrongFileSelection_LookalikeNeighbour',
  'TestBuildFilelist_IsNulDelimited',
  'TestBuildRestore_HostileFilenamesRoundTrip',
  'TestBuildRestore_UnicodeRoundTrip',
];

const [file] = process.argv.slice(2);
if (!file) throw Error('Usage: node scripts/ci/go-known-failures.mjs GO_TEST_JSON_FILE');
const events = fs.readFileSync(file, 'utf8').split(/\r?\n/).filter(l => l.startsWith('{')).map(l => JSON.parse(l));
const final = new Map(); // "pkg test" -> last terminal action
const packages = new Map();
for (const e of events) {
  if (!['pass', 'fail', 'skip'].includes(e.Action)) continue;
  if (e.Test) final.set(e.Package + ' ' + e.Test, { pkg: e.Package, test: e.Test, action: e.Action });
  else packages.set(e.Package, e.Action);
}
const top = [...final.values()].filter(r => !r.test.includes('/'));
const count = a => top.filter(r => r.action === a).length;
const failed = top.filter(r => r.action === 'fail').map(r => r.test);
const skipped = top.filter(r => r.action === 'skip');
const status = name => top.find(r => r.test === name)?.action ?? 'did not run';

console.log(`Top-level tests: ${count('pass')} pass, ${count('fail')} fail, ${count('skip')} skip; packages: ${[...packages].map(([p, a]) => p + '=' + a).join(', ') || 'none reported'}`);
console.log(`Skipped (not passes): ${skipped.length}`);
for (const s of skipped) console.log('  SKIP ' + s.test);

const unexpected = failed.filter(t => !known.includes(t));
const missing = known.filter(t => !failed.includes(t));
const failingPackagesWithoutTests = [...packages].filter(([p, a]) => a === 'fail' && !top.some(r => r.pkg === p && r.action === 'fail')).map(([p]) => p);
for (const t of known) console.log(`  KNOWN ${failed.includes(t) ? 'failed as expected' : 'NOT FAILED (' + status(t) + ')'}: ${t}`);
for (const t of unexpected) console.log('  NEW FAILURE: ' + t);
for (const p of failingPackagesWithoutTests) console.log('  PACKAGE FAILED WITHOUT A FAILING TEST: ' + p);
if (!events.length) console.log('  NO go test -json EVENTS');

if (unexpected.length || missing.length || failingPackagesWithoutTests.length || !events.length) {
  console.error(`Gate failed: ${unexpected.length} new failure(s), ${missing.length} known failure(s) did not fail, ${failingPackagesWithoutTests.length} package(s) failed without a failing test.`);
  process.exit(1);
}
console.log(`Gate passed: failures are exactly the ${known.length} known Windows failures.`);
