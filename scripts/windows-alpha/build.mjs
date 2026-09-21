// Developer-only build. Never packaged or run by testers. No downloads.
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import assert from 'node:assert/strict';
import { runtimeFiles, launcherFiles, dependencies } from './package-files.mjs';
import { makeZip } from './zip.mjs';

const scripts = path.dirname(fileURLToPath(import.meta.url)), repo = path.resolve(scripts, '../..');
const base = 'bfbce891df78d529c6be2d2912dc8443597c007e', implementation = '8eb178bcb6f8f7367d2cb9aa75d2f4059d92a85c';
const [output, go, moduleCache] = process.argv.slice(2);
if (process.argv.length !== 5 || [output, go, moduleCache].some(p => !p || !path.isAbsolute(p))) throw Error('Usage: node build.mjs ABS_NEW_TASK_BUILD ABS_INSTALLED_GO_EXE ABS_EXISTING_MODULE_CACHE');
assert.equal(process.platform, 'win32'); assert.equal(process.arch, 'x64');
const taskBase = fs.realpathSync(path.join(process.env.LOCALAPPDATA, 'ObeliskDev'));
const relative = path.relative(taskBase, path.resolve(output)); assert.ok(relative && !relative.startsWith('..') && !path.isAbsolute(relative));
assert.ok(!fs.existsSync(output), 'Build directory already exists; no replacement or automatic retry');
fs.mkdirSync(output);
const source = path.join(output, 'source'), stage = path.join(output, 'stage'); fs.mkdirSync(source); fs.mkdirSync(stage);
const events = [], sha = b => createHash('sha256').update(b).digest('hex');
function command(exe, args, cwd, env = process.env) {
  const started = new Date().toISOString(), t = Date.now();
  const result = spawnSync(exe, args, { cwd, env, windowsHide: true, maxBuffer: 32 * 1024 * 1024, timeout: 120000 });
  events.push({ exe, args, cwd, started, durationMs: Date.now() - t, exit: result.status, error: result.error?.code });
  fs.writeFileSync(path.join(output, 'commands.json'), JSON.stringify(events, null, 2));
  fs.writeFileSync(path.join(output, `command-${events.length}.stdout`), result.stdout || '');
  fs.writeFileSync(path.join(output, `command-${events.length}.stderr`), result.stderr || '');
  if (result.status !== 0) throw Error(`Command ${events.length} failed (${result.status}, ${result.error?.code ?? 'see stderr'}). Stop; do not recreate a security-blocked output.`);
  return result.stdout;
}
const git = (...args) => command('git', args, repo);
assert.equal(git('rev-parse', 'HEAD').toString().trim(), base);
assert.equal(git('rev-parse', base + '^').toString().trim(), implementation);
const tracked = git('ls-tree', '-r', '--name-only', base).toString().trim().split('\n');
const sourceFiles = tracked.filter(n => (/^[^/]+\.go$/.test(n) && !n.endsWith('_test.go')) || ['go.mod', 'go.sum', 'formats.json', 'docs/COMPARISON.md', 'docs/RESTORE_RUNBOOK.md', 'escrow_manifest.json', 'escrow/obelisk-src.tar.gz'].includes(n) || n.startsWith('ui/'));
const sourceManifest = [];
function write(dir, name, bytes) { fs.mkdirSync(path.dirname(path.join(dir, name)), { recursive: true }); fs.writeFileSync(path.join(dir, name), bytes, { flag: 'wx' }); }
for (const name of sourceFiles) { const bytes = git('show', base + ':' + name); write(source, name, bytes); sourceManifest.push({ path: name, bytes: bytes.length, sha256: sha(bytes) }); }
fs.writeFileSync(path.join(output, 'build-source.json'), JSON.stringify(sourceManifest, null, 2));
const env = { ...process.env, GOTOOLCHAIN: 'local', GOPROXY: 'off', GOSUMDB: 'off', GOFLAGS: '-mod=readonly', CGO_ENABLED: '0', GOOS: 'windows', GOARCH: 'amd64', GOMODCACHE: moduleCache, GOCACHE: path.join(output, 'go-cache'), TEMP: path.join(output, 'go-temp'), TMP: path.join(output, 'go-temp') }; delete env.GOTMPDIR;
fs.mkdirSync(env.TEMP);
const goVersion = command(go, ['version'], source, env).toString().trim();
const label = '0.9.0-dev-inventory-package.' + base.slice(0, 12);
fs.mkdirSync(path.join(stage, 'bin'));
command(go, ['build', '-trimpath', '-buildvcs=false', '-ldflags', '-X main.appVersion=' + label, '-o', path.join(stage, 'bin/obelisk.exe'), '.'], source, env);
const buildInfo = command(go, ['version', '-m', path.join(stage, 'bin/obelisk.exe')], source, env).toString();
fs.writeFileSync(path.join(output, 'binary-build-info.txt'), buildInfo);
const entries = ['bin/obelisk.exe'];
for (const name of runtimeFiles) { write(stage, 'preview/' + name, git('show', base + ':scripts/gui-preview/' + name)); entries.push('preview/' + name); }
for (const name of launcherFiles) { write(stage, name, fs.readFileSync(path.join(scripts, name))); entries.push(name); }
write(stage, 'LICENSE', git('show', base + ':LICENSE')); entries.push('LICENSE');
let notices = 'Third-party code linked into this full Obelisk binary. No Node/browser/helper executables are bundled.\n\n';
for (const [module, license] of dependencies) notices += module + '\n' + fs.readFileSync(path.join(moduleCache, module, license), 'utf8') + '\n\n';
notices += 'Go standard library/toolchain runtime\n' + fs.readFileSync(path.resolve(go, '../../LICENSE'), 'utf8');
write(stage, 'THIRD-PARTY-NOTICES.txt', Buffer.from(notices)); entries.push('THIRD-PARTY-NOTICES.txt');
const scriptNames = [...launcherFiles, 'build.mjs', 'zip.mjs', 'package-files.mjs'];
const scriptIdentities = scriptNames.map(name => ({ path: 'scripts/windows-alpha/' + name, sha256: sha(fs.readFileSync(path.join(scripts, name))), revision: 'uncommitted packaging candidate on ' + base }));
const manifest = { packageID: label + '-windows-amd64-local', unsigned: true, status: 'DEVELOPER_ALPHA_PACKAGING_CANDIDATE', sourceCommit: base, acceptedRuntimeImplementation: implementation, scriptIdentities, target: 'windows/amd64', build: { goVersion, node: process.version, host: os.release(), flags: ['-trimpath', '-buildvcs=false', '-ldflags=-X main.appVersion=' + label], CGO_ENABLED: '0' }, prerequisites: { node: '24.x x64, externally installed', browser: 'externally installed modern browser; Chrome exercised', powershell: 'Windows PowerShell for Launch.ps1; obey existing script policy' }, files: entries.map(name => { const bytes = fs.readFileSync(path.join(stage, name)); return { path: name, bytes: bytes.length, sha256: sha(bytes) }; }) };
write(stage, 'package-manifest.json', Buffer.from(JSON.stringify(manifest, null, 2))); entries.push('package-manifest.json');
const zip = makeZip(entries.map(name => ({ name, data: fs.readFileSync(path.join(stage, name)) })));
const zipPath = path.join(output, manifest.packageID + '.zip'); fs.writeFileSync(zipPath, zip, { flag: 'wx' });
fs.writeFileSync(path.join(output, 'artifact.json'), JSON.stringify({ zipPath, bytes: zip.length, sha256: sha(zip), manifestSHA256: sha(fs.readFileSync(path.join(stage, 'package-manifest.json'))), entries, buildEnvironment: envSubset(env) }, null, 2));
console.log(JSON.stringify({ zipPath, sha256: sha(zip), entries: entries.length, goVersion }));
function envSubset(e) { return Object.fromEntries(['GOTOOLCHAIN','GOPROXY','GOSUMDB','GOFLAGS','CGO_ENABLED','GOOS','GOARCH','GOMODCACHE','GOCACHE','TEMP','TMP'].map(k => [k,e[k]])); }
