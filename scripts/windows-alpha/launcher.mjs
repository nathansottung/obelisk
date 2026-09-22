import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import { spawn } from 'node:child_process';
import { tutorialFiles, comparisonTutorialFiles } from './tutorial-fixtures.mjs';

const root = path.dirname(fileURLToPath(import.meta.url));
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const within = (parent, child) => { const r = path.relative(parent, child); return r === '' || (!r.startsWith('..') && !path.isAbsolute(r)); };
const help = `Unsigned Windows developer-alpha PACKAGING CANDIDATE; generated sources only.
Existing Node.js 24 x64 and a browser are required. No downloads or installation.
Actions (run through Launch.ps1, or node launcher.mjs):
  help
  check
  generate ABS_NEW_WORKSPACE
  generate-pair ABS_NEW_WORKSPACE
  inventory ABS_WORKSPACE NEW_NAME.json [--ignore-ds-store]
  view ABS_SYNTHETIC_CATALOG [ABS_SECOND_SYNTHETIC_CATALOG]
  static
Choose a new workspace beneath your LOCALAPPDATA/ObeliskDev. Generation,
inventory (which reads generated source files), and viewing are separate actions.
The viewer reads but does not modify the selected catalog; recorded source/media
paths are not opened. See QUICKSTART.md for limits, compatibility and stop/reopen.
The packaged binary contains only the inventory and read-only viewer modes.
This launcher is not a sandbox.`;

async function verifyPackage() {
  const manifest = JSON.parse(await fs.readFile(path.join(root, 'package-manifest.json'), 'utf8'));
  for (const item of manifest.files) {
    if (!item.path || path.isAbsolute(item.path) || !within(root, path.resolve(root, item.path))) throw Error('Invalid package entry');
    let bytes;
    try { bytes = await fs.readFile(path.join(root, item.path)); }
    catch { throw Error(`Missing package file: ${item.path}. Stop and report; no fallback or download.`); }
    if (bytes.length !== item.bytes || sha(bytes) !== item.sha256) throw Error(`Package identity mismatch: ${item.path}. Stop and report; do not bypass a detection.`);
  }
  return manifest;
}

async function workspacePath(value, creating = false) {
  if (!value || !path.isAbsolute(value) || !/^[a-z]:[\\/]/i.test(value) || !process.env.LOCALAPPDATA) throw Error('Explicit ordinary local absolute workspace required');
  const boundary = path.join(process.env.LOCALAPPDATA, 'ObeliskDev');
  const selected = path.resolve(value);
  if (selected === boundary || !within(boundary, selected) || within(root, selected) || within(selected, root)) throw Error('Choose a separate NEW workspace beneath LOCALAPPDATA/ObeliskDev, outside the package');
  // Only this conventional development root may be created automatically.
  if (creating) await fs.mkdir(boundary).catch(e => { if (e.code !== 'EEXIST') throw e; });
  for (let current = creating ? path.dirname(selected) : selected; ; current = path.dirname(current)) {
    const info = await fs.lstat(current);
    if (info.isSymbolicLink() || !info.isDirectory()) throw Error('Workspace ancestors must be ordinary directories, without links');
    if (path.dirname(current) === current) break;
  }
  const actualBoundary = await fs.realpath(boundary), actualParent = await fs.realpath(creating ? path.dirname(selected) : selected);
  if (!within(actualBoundary, actualParent)) throw Error('Workspace resolves outside ObeliskDev');
  return selected;
}

async function generate(value, definitions = tutorialFiles, variant = 'single-v1') {
  const workspace = await workspacePath(value, true);
  await fs.mkdir(workspace); // EEXIST refuses even an empty previous workspace.
  await fs.mkdir(path.join(workspace, 'source'));
  await fs.mkdir(path.join(workspace, 'catalogs'));
  const files = [];
  for (const [name, text] of Object.entries(definitions)) {
    const target = path.join(workspace, 'source', name), bytes = Buffer.from(text);
    await fs.mkdir(path.dirname(target), { recursive: true });
    await fs.writeFile(target, bytes, { flag: 'wx' });
    files.push({ path: name, bytes: bytes.length, sha256: sha(bytes) });
  }
  await fs.writeFile(path.join(workspace, 'tutorial-workspace.json'), JSON.stringify({ kind: 'obelisk-generated-tutorial-v1', variant, files }, null, 2), { flag: 'wx' });
  console.log(`Generated ${files.length} expendable files in ${workspace}. No inventory has run.`);
}

async function generatePair(value) {
  const workspace = await workspacePath(value, true);
  await fs.mkdir(workspace); // Never merge, replace or clean an earlier pair.
  for (const variant of ['ALPHA', 'BETA']) await generate(path.join(workspace, variant), comparisonTutorialFiles[variant], 'comparison-v1-' + variant);
  console.log('ALPHA and BETA generated. Explicit inventory actions are still required; no catalog or comparison has run.');
}

async function run(executable, args) {
  console.log(JSON.stringify({ executable, args, packageRoot: root }));
  const child = spawn(executable, args, { windowsHide: true, stdio: 'inherit', shell: false });
  // In an interactive console the child receives the same Ctrl+C event. Keep
  // the wrapper alive to wait; do not turn SIGINT into a Windows forced kill.
  const waitForChild = () => {};
  process.on('SIGINT', waitForChild);
  const result = await new Promise((resolve, reject) => { child.once('error', reject); child.once('exit', (code, signal) => resolve({ code, signal })); });
  process.off('SIGINT', waitForChild);
  console.log('Packaged child waited: ' + JSON.stringify(result));
  process.exitCode = result.code ?? 1; // Never turn published:true/nonzero into success.
}

async function main() {
  const [action = 'help', ...args] = process.argv.slice(2);
  if (process.platform !== 'win32' || process.arch !== 'x64' || Number(process.versions.node.split('.')[0]) !== 24) throw Error('Supported prerequisite: Windows x64 with existing Node.js 24 x64. No automatic installation.');
  if (action === 'help' && args.length === 0) { console.log(help); return; }
  if (!['check', 'generate', 'generate-pair', 'inventory', 'view', 'static'].includes(action)) throw Error(help);
  const manifest = await verifyPackage();
  if (action === 'check' && args.length === 0) { console.log(JSON.stringify({ packageID: manifest.packageID, node: process.version, arch: process.arch, root, filesVerified: manifest.files.length })); return; }
  if (action === 'generate' && args.length === 1) { await generate(args[0]); return; }
  if (action === 'generate-pair' && args.length === 1) { await generatePair(args[0]); return; }
  const adapter = path.join(root, 'bin', 'obelisk.exe');
  if (action === 'inventory' && (args.length === 2 || (args.length === 3 && args[2] === '--ignore-ds-store'))) {
    const workspace = await workspacePath(args[0]);
    if (!/^[a-zA-Z0-9][a-zA-Z0-9_-]*\.json$/.test(args[1])) throw Error('Use a new simple catalog filename, for example off-2.json');
    const marker = JSON.parse(await fs.readFile(path.join(workspace, 'tutorial-workspace.json'), 'utf8'));
    if (marker.kind !== 'obelisk-generated-tutorial-v1') throw Error('Select an explicitly generated tutorial workspace');
    console.log('Inventory reads the explicitly selected generated source. Existing output must refuse; preserve published status even on a nonzero exit.');
    await run(adapter, ['--gui-disposable-inventory', ...(args[2] ? ['--ignore-ds-store'] : []), path.join(workspace, 'source'), path.join(workspace, 'catalogs', args[1])]);
    return;
  }
  if ((action === 'view' && [1, 2].includes(args.length) && args.every(p => path.isAbsolute(p))) || (action === 'static' && args.length === 0)) {
    if (action === 'view') {
      const selected = new Set();
      for (const file of args) {
        const info = await fs.lstat(file);
        if (!info.isFile() || info.isSymbolicLink()) throw Error('Select an existing regular synthetic catalog file, not a link');
        const actual = (await fs.realpath(file)).toLowerCase();
        if (selected.has(actual)) throw Error('Duplicate catalog input refused; choose two different recorded artifacts');
        selected.add(actual);
      }
    }
    if (action === 'view') console.log('Read-only VIEWER: selected synthetic catalog is read, not modified; recorded source/media paths are not opened.');
    else console.log('INTENTIONAL STATIC DEMO: all records are synthetic samples, not a loaded catalog.');
    await run(process.execPath, [path.join(root, 'preview', 'server.mjs'), '0', ...(action === 'view' ? [...args.flatMap(file => ['--catalog', file]), '--adapter', adapter] : [])]);
    return;
  }
  throw Error(help);
}
main().catch(error => { console.error('Developer-alpha action refused: ' + error.message); process.exitCode = 1; });
