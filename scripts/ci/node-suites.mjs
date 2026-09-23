// CI entry point for the Node suites. Builds the native reader and the two test
// doubles, generates every gui-preview fixture set in a NEW directory beneath
// %LOCALAPPDATA%\ObeliskDev (the catalog adapter only accepts inputs there), runs
// all eight scripts/gui-preview suites, and syntax-checks scripts/windows-alpha.
// Windows only. The windows-alpha suites are not run here: package.test.mjs needs
// a built ZIP and boundary.test.mjs needs an extracted package.
//
// Usage (from the repository root): node scripts/ci/node-suites.mjs [GO_EXE]
// Exit 0 only when every step succeeds and no suite reports a failure,
// cancellation or skip.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const go = process.argv[2] || 'go';
if (process.platform !== 'win32') throw Error('Windows only: the catalog adapter requires inputs beneath LOCALAPPDATA\\ObeliskDev');
if (!process.env.LOCALAPPDATA) throw Error('LOCALAPPDATA is not set');
const obeliskDev = path.join(process.env.LOCALAPPDATA, 'ObeliskDev');
fs.mkdirSync(obeliskDev, { recursive: true });
const stamp = new Date().toISOString().replace(/[-:]/g, '').replace(/\..*/, '').replace('T', '-');
const root = path.join(obeliskDev, `ci-node-${stamp}-${crypto.randomBytes(3).toString('hex')}`);
fs.mkdirSync(root);
const out = path.join(root, 'output');
fs.mkdirSync(out);
const sha = b => crypto.createHash('sha256').update(b).digest('hex');
const at = (...p) => path.join(root, ...p);
console.log('Fixture root: ' + root);

function run(label, exe, args, opts = {}) {
  const started = Date.now();
  const r = spawnSync(exe, args, { cwd: repo, encoding: 'utf8', maxBuffer: 64 << 20, windowsHide: true, ...opts });
  console.log(`[${label}] exit ${r.status} (${Date.now() - started} ms)`);
  if (r.error) throw r.error;
  if (r.status !== 0 && !opts.allowFailure) throw Error(`${label} failed:\n${r.stdout}\n${r.stderr}`);
  return r;
}

// 1. Native reader and the two fault-emitting test doubles.
const reader = path.join(out, 'reader.exe'), double = path.join(out, 'double.exe'), comparisonDouble = path.join(out, 'comparison-double.exe');
run('build reader', go, ['build', '-o', reader, '.']);
run('build double', go, ['build', '-o', double, 'scripts/gui-preview/testdata/reader-double.go']);
run('build comparison double', go, ['build', '-o', comparisonDouble, 'scripts/gui-preview/testdata/comparison-reader-double.go']);

// 2. Go fixture generators. Each is skipped unless its output variable is set, so
// a skip here means missing fixtures and fails the run.
const dirs = { catalog: at('catalog-inputs'), correction: at('correction-inputs'), multi: at('multi-inputs'), comparison: at('comparison-inputs') };
const generators = ['TestGUICatalogGenerateFixtures', 'TestGUICatalogCorrectionFixtures', 'TestGUIMultiSnapshotFixtures', 'TestGUIComparisonFixtures'];
const gen = run('go fixture generators', go, ['test', '-count=1', '-json', '-run', '^(' + generators.join('|') + ')$', '.'], {
  env: { ...process.env, OBELISK_GUI_FIXTURE_OUTPUT: dirs.catalog, OBELISK_GUI_CORRECTION_FIXTURES: dirs.correction, OBELISK_GUI_MULTI_FIXTURES: dirs.multi, OBELISK_GUI_COMPARISON_FIXTURES: dirs.comparison },
  allowFailure: true,
});
const genResults = Object.fromEntries(gen.stdout.split('\n').filter(l => l.startsWith('{')).map(l => JSON.parse(l)).filter(e => e.Test && ['pass', 'fail', 'skip'].includes(e.Action)).map(e => [e.Test, e.Action]));
for (const t of generators) if (genResults[t] !== 'pass') throw Error(`${t}: ${genResults[t] ?? 'did not run'}\n${gen.stdout.slice(-4000)}`);

// 3. Fault sets for the two-reader suites. The doubles read projection.json beside
// the input and choose a fault by file name, so the named files are placeholders.
const write = (file, data) => { fs.mkdirSync(path.dirname(file), { recursive: true }); fs.writeFileSync(file, data, { flag: 'wx' }); };
const multi = JSON.parse(fs.readFileSync(path.join(dirs.correction, 'projection.json'), 'utf8'));
multi.catalog.digest = 'f'.repeat(64); // distinct digest so the pair is not refused as a duplicate
write(at('multi-faults', 'projection.json'), JSON.stringify(multi));
for (const n of ['query-failed', 'unknown-id', 'late']) write(at('multi-faults', n + '.json'), 'protocol simulation');
const snapshots = [];
for (const [i, name] of ['a', 'b'].entries()) {
  const file = path.join(dirs.comparison, name + '.json');
  const p = run('project ' + name, reader, ['--gui-catalog-readonly', file], { input: '' });
  const projection = JSON.parse(p.stdout.trim());
  snapshots.push(projection);
  const dir = at('comparison-faults', i ? 'B' : 'A');
  write(path.join(dir, 'projection.json'), JSON.stringify(projection));
  for (const n of ['good', 'late', 'timeout', 'partial', 'truncated', 'unknown', 'dies']) write(path.join(dir, n + '.json'), 'protocol simulation');
}
for (let i = 0; i < 2; i++) {
  // Response-cap simulation: native-readable shape, not a native 4 MiB input.
  const p = structuredClone(snapshots[i]);
  p.catalog.files = Array.from({ length: 500 }, (_, j) => ({ ...p.catalog.files[0], id: String(j + 1), path: i + '/' + j + '/' + '\u0001'.repeat(4000) }));
  p.catalog.comparisonFrame.recordCount = 500;
  write(at('comparison-faults', 'big' + i, 'projection.json'), JSON.stringify(p));
  write(at('comparison-faults', 'big' + i, 'good.json'), 'protocol response cap simulation');
}
for (let i = 0; i < 2; i++) {
  // Two real producer observations beside the synthetic comparison inputs.
  const src = at('off-sources', String(i));
  write(path.join(src, 'sentinel.txt'), 'synthetic ' + i);
  run('produce off-' + i, reader, ['--gui-disposable-inventory', src, path.join(dirs.comparison, 'off-' + i + '.json')]);
}

// 4. Producer snapshots for the inventory-correction suite: OFF/ON over one
// generated tree, empty, all-excluded and zero-exclusion sources, plus a foreign
// native-shaped catalog whose names are data only.
const snap = at('snapshots');
fs.mkdirSync(snap);
const names = ['same.txt', 'nested space/same.txt', 'empty.txt', 'equal one.txt', 'nested space/equal two.txt', '.DS_Store', 'nested space/.DS_Store', '.DS_Store.bak', 'photo.DS_Store', '._.DS_Store', 'a/.ds_store', 'b/.DS_STORE', 'c/.dstore', 'directory/.DS_Store/valuable.txt', '.hidden', '__MACOSX/keep', '.Spotlight-V100/keep', '.Trashes/keep', 'Thumbs.db', 'desktop.ini', 'photo.xmp', 'photo.aae', "O'Brien [x](y) &+%#.txt", 'caf\u00e9.txt', 'cafe\u0301.txt', '\u96ea-\ud83d\ude80.txt', 'literal\ufffd.txt', '-leading.txt'];
const tree = new Map(names.map(n => [n, Buffer.from(n === 'empty.txt' ? '' : n.includes('equal') ? 'same independent bytes' : 'generated fixture: ' + n)]));
const source = (label, values) => {
  const dir = at('fixtures', label);
  fs.mkdirSync(dir, { recursive: true });
  for (const [p, b] of values) {
    const file = path.join(dir, ...p.split('/'));
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, b, { flag: 'wx' });
    fs.utimesSync(file, new Date('2026-02-03T04:05:06Z'), new Date('2026-02-03T04:05:06Z'));
  }
  return dir;
};
const main = source("- Generated caf\u00e9 O'Brien &+%#", tree);
const cases = [['off', main, false], ['on', main, true], ['empty', source('EMPTY', new Map()), true], ['all-excluded', source('ALL EXCLUDED', new Map([['.DS_Store', Buffer.from('keep excluded source')]])), true], ['on-zero', source('ZERO EXCLUSIONS', new Map([['.dstore', Buffer.from('not excluded')]])), true]];
for (const [name, src, ignore] of cases) {
  const r = run('produce ' + name, reader, ['--gui-disposable-inventory', ...(ignore ? ['--ignore-ds-store'] : []), src, path.join(snap, name + '.json')]);
  if (JSON.parse(r.stdout.trim().split('\n').at(-1)).published !== true) throw Error(name + ' was not published');
}
const off = JSON.parse(fs.readFileSync(path.join(snap, 'off.json'), 'utf8'));
const foreign = structuredClone(off);
foreign.audit = []; foreign.collections[0].name = 'Foreign names as data'; foreign.folders[0].path = '/foreign/never-opened'; foreign.files = [];
const foreignNames = ['line\nname.txt', 'line\rname.txt', 'line\r\nname.txt', 'linename.txt', String.raw`line\nname.txt`, String.raw`line\rname.txt`, '\nleading and trailing\r', '\ttab\t', ' space ', "O'Brien &+%#.txt", String.raw`C:\folder\new.txt`, 'literal\ufffd.txt', '\ud83d\ude80.txt', 'quote"back\\slash'];
const oracleFiles = foreignNames.map((p, i) => {
  const b = Buffer.from('distinct expected evidence ' + i);
  foreign.files.push({ ...off.files[0], id: i + 1, rel_path: p, size_bytes: b.length, hash: sha(b), blake3: '' });
  return { id: String(i + 1), path: p, bytes: b.length, sha256: sha(b) };
});
foreign.next_id.file = oracleFiles.length;
write(path.join(snap, 'foreign.json'), JSON.stringify(foreign, null, 2));
write(path.join(snap, 'foreign-oracle.json'), JSON.stringify({ label: 'Foreign names as data', source: foreign.folders[0].path, scope: null, files: oracleFiles }, null, 2));

// 5. Scope-key cases for the inventory-scope-keys suite, each checked against the
// native reader before use.
const empty = JSON.parse(fs.readFileSync(path.join(snap, 'empty.json'), 'utf8'));
const canonical = empty.audit[0].detail;
const scopeCases = [];
const addCase = (name, raw, ok) => { const file = path.join(snap, 'scope-' + name + '.json'); write(file, raw); scopeCases.push({ name, file, ok, sha256: sha(raw) }); };
const detail = (name, s, ok = false) => { const c = structuredClone(empty); c.audit[0].detail = s; addCase(name, Buffer.from(JSON.stringify(c)), ok); };
detail('reviewer-missing-entries-case-alias', canonical.replace('{"version":1,', '{"Version":2,"version":1,').replace('"entries":0,', ''));
detail('reviewer-missing-excluded-case-alias', canonical.replace('"excludedFiles":0,', '').replace('"complete":true', '"Complete":false,"complete":true'));
for (const key of ['version', 'policy', 'entries', 'regularFiles', 'includedFiles', 'excludedFiles', 'readBytes', 'complete']) detail('alias-' + key, canonical.replace('"' + key + '":', '"' + key[0].toUpperCase() + key.slice(1) + '":'));
detail('encoded-key', canonical.replace('"entries"', '"\\u0065ntries"'), true);
detail('encoded-case', canonical.replace('"entries"', '"\\u0045ntries"'));
detail('encoded-duplicate', canonical.replace('"entries":0', '"entries":0,"\\u0065ntries":0'));
detail('conflict-before', canonical.replace('"complete":true', '"complete":false,"complete":true'));
detail('conflict-after', canonical.replace('"complete":true', '"complete":true,"complete":false'));
const event = JSON.stringify(empty.audit[0]);
const envelope = structuredClone(off); delete envelope.audit;
const prefix = JSON.stringify(envelope).slice(0, -1); // valid records precede the final audit member
for (const [name, members, ok] of [
  ['late-wrapper-alias', '"Audit":[' + event + ']', false],
  ['late-duplicate-before', '"audit":[null],"audit":' + JSON.stringify(off.audit), false],
  ['late-duplicate-after', '"audit":' + JSON.stringify(off.audit) + ',"audit":[]', false],
  ['event-alias', '"audit":[' + JSON.stringify(off.audit[0]).replace('"detail":', '"Detail":') + ']', false],
  ['event-encoded-duplicate', '"audit":[' + JSON.stringify(off.audit[0]).replace('"action":', '"\\u0061ction":"GUI_DISPOSABLE_INVENTORY_V1","action":') + ']', false],
  ['legacy-absent', '', true], ['legacy-null', '"audit":null', true],
  ['escaped-wrapper', '"\\u0061udit":' + JSON.stringify(off.audit), true],
]) addCase(name, Buffer.from(prefix + (members ? ',' + members : '') + '}'), ok);
for (const c of scopeCases) {
  const r = spawnSync(reader, ['--gui-catalog-readonly', c.file], { input: '{"text":""}\n', encoding: 'utf8', windowsHide: true, timeout: 10000 });
  if (r.status !== (c.ok ? 0 : 1) || JSON.parse(r.stdout.trim().split('\n')[0]).ok !== c.ok) throw Error(`scope case ${c.name}: native reader exit ${r.status}, expected ${c.ok ? 0 : 1}\n${r.stderr}`);
}
write(at('scope', 'scope-cases.json'), JSON.stringify(scopeCases, null, 2));
console.log(`Fixtures ready: ${scopeCases.length} scope cases, ${cases.length} producer snapshots`);

// 6. The eight gui-preview suites, one process each, TAP counts parsed.
const env = {
  ...process.env,
  OBELISK_GUI_TEST_INPUTS: dirs.catalog, OBELISK_GUI_CORRECTION_INPUTS: dirs.correction, OBELISK_GUI_MULTI_FIXTURES: dirs.multi, OBELISK_GUI_COMPARISON_FIXTURES: dirs.comparison,
  OBELISK_GUI_MULTI_FAULTS: at('multi-faults'), OBELISK_GUI_COMPARISON_FAULTS: at('comparison-faults'),
  OBELISK_GUI_INVENTORY_FIX_INPUTS: snap, OBELISK_GUI_SCOPE_KEY_INPUTS: at('scope'),
  OBELISK_GUI_TEST_ADAPTER: reader, OBELISK_GUI_TEST_DOUBLE: double, OBELISK_GUI_COMPARISON_DOUBLE: comparisonDouble,
};
const suites = ['stop-command', 'preview', 'catalog', 'catalog-correction', 'inventory-correction', 'inventory-scope-keys', 'multi-snapshot', 'recorded-comparison'];
const summary = [];
for (const suite of suites) {
  const file = `scripts/gui-preview/${suite}.test.mjs`;
  const r = run('suite ' + suite, process.execPath, ['--test', '--test-isolation=none', '--test-reporter=tap', file], { env, allowFailure: true, timeout: 15 * 60 * 1000 });
  fs.writeFileSync(path.join(out, `node-${suite}.tap`), r.stdout + r.stderr);
  const count = k => Number(r.stdout.match(new RegExp('^# ' + k + ' (\\d+)$', 'm'))?.[1] ?? NaN);
  const row = { suite, exit: r.status, tests: count('tests'), pass: count('pass'), fail: count('fail'), cancelled: count('cancelled'), skipped: count('skipped'), todo: count('todo') };
  summary.push(row);
  if (row.exit !== 0 || row.fail || row.cancelled || row.skipped || !(row.tests > 0)) console.log(r.stdout.slice(-6000) + r.stderr.slice(-2000));
}

// 7. windows-alpha: syntax only (see header).
const alpha = fs.readdirSync(path.join(repo, 'scripts/windows-alpha')).filter(f => f.endsWith('.mjs'));
for (const f of alpha) run('node --check windows-alpha/' + f, process.execPath, ['--check', path.join('scripts/windows-alpha', f)]);

console.table(summary);
fs.writeFileSync(path.join(out, 'node-summary.json'), JSON.stringify(summary, null, 2));
const bad = summary.filter(r => r.exit !== 0 || r.fail || r.cancelled || r.skipped || !(r.tests > 0));
if (bad.length) { console.error('Suites not clean: ' + bad.map(r => r.suite).join(', ')); process.exit(1); }
console.log(`All ${suites.length} gui-preview suites passed (${summary.reduce((n, r) => n + r.pass, 0)} tests); ${alpha.length} windows-alpha scripts syntax-checked.`);
