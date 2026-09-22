import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { spawn } from 'node:child_process';
import childProcess from 'node:child_process';
import { syncBuiltinESMExports } from 'node:module';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { createHash, randomUUID } from 'node:crypto';
import { runtimeFiles, launcherFiles } from './package-files.mjs';

const scriptRoot = path.dirname(fileURLToPath(import.meta.url));
const build = process.env.OBELISK_ALPHA_BUILD, evidence = process.env.OBELISK_ALPHA_TEST;
if (!build || !evidence || !path.isAbsolute(evidence)) throw Error('Provide explicit build and NEW test evidence directories');
await fs.mkdir(evidence);
const artifact = JSON.parse(await fs.readFile(path.join(build, 'artifact.json'), 'utf8'));
const ps = path.join(process.env.SystemRoot, 'System32/WindowsPowerShell/v1.0/powershell.exe');
const cwd = path.join(evidence, 'unrelated cwd'); await fs.mkdir(cwd);
const env = Object.fromEntries(['SystemRoot','WINDIR','LOCALAPPDATA','APPDATA','USERPROFILE','TEMP','TMP','COMSPEC','PATHEXT'].map(k => [k,process.env[k]]));
env.PATH = path.dirname(process.execPath) + path.delimiter + path.join(process.env.SystemRoot, 'System32');
const packages = [path.join(evidence, 'extracted-one'), path.join(evidence, "relocated café O'Brien & +%#")];
const workspaces = [path.join(evidence, 'tutorial-one'), path.join(evidence, 'tutorial café two')];
const sha = b => createHash('sha256').update(b).digest('hex');
const events = []; let sequence = 0;
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
async function save() { await fs.writeFile(path.join(evidence, 'commands.json'), JSON.stringify(events, null, 2)); }
function child(exe, args, overrides = {}, verbatim = false) {
  const event = { sequence: ++sequence, exe, args, cwd, started: new Date().toISOString(), forced: false };
  const proc = spawn(exe, args, { cwd, env: { ...env, ...overrides }, windowsHide: true, stdio: ['pipe','pipe','pipe'], shell: false, windowsVerbatimArguments: verbatim });
  let stdout = '', stderr = ''; proc.stdout.on('data', b => stdout += b); proc.stderr.on('data', b => stderr += b);
  // Launch.cmd runs Node under cmd.exe; end the whole tree, not only cmd.exe.
  const timeout = setTimeout(() => { event.forced = true; childProcess.spawnSync(path.join(process.env.SystemRoot, 'System32/taskkill.exe'), ['/pid', String(proc.pid), '/t', '/f']); }, 30000);
  const ended = new Promise((resolve, reject) => { proc.once('error', reject); proc.once('close', async (code, signal) => { clearTimeout(timeout); Object.assign(event, { code, signal, finished: new Date().toISOString(), stdout, stderr }); events.push(event); await save(); resolve(event); }); });
  return { proc, ended, text: () => stdout, event };
}
async function finite(exe, args, expected = 0, overrides, verbatim = false) { const c = child(exe, args, overrides, verbatim); c.proc.stdin.end(); const r = await c.ended; assert.equal(r.code, expected, r.stderr + r.stdout); assert.equal(r.forced, false); return r; }
// The same command line a tester types in Command Prompt: "…\Launch.cmd" "arg" …
const cmdExe = path.join(process.env.SystemRoot, 'System32/cmd.exe');
const cmdLine = (pkg, args) => ['/d', '/s', '/c', '"' + [path.join(pkg, 'Launch.cmd'), ...args].map(a => '"' + a + '"').join(' ') + '"'];
const launch = (pkg, args, expected = 0, overrides) => finite(cmdExe, cmdLine(pkg, args), expected, overrides, true);
async function tree(root) { const result = {}; async function visit(dir) { for (const e of await fs.readdir(dir, { withFileTypes: true })) { const p = path.join(dir,e.name); if(e.isDirectory())await visit(p);else result[path.relative(root,p)] = sha(await fs.readFile(p)); } } await visit(root); return result; }
async function start(pkg, args) {
  const c = child(cmdExe, cmdLine(pkg, args), {}, true);
  for(let i=0;i<600;i++){const m=c.text().match(/http:\/\/127\.0\.0\.1:\d+\//);if(m)return {...c,url:m[0]};if(c.proc.exitCode!==null)throw Error(c.text());await delay(20);}
  throw Error('No packaged viewer URL');
}
async function stop(c, readerCode = 0) {
  c.proc.stdin.write('st'); await delay(80); assert.equal((await fetch(c.url)).status,200);
  c.proc.stdin.write('op\n'); // Keep stdin open until decisive natural exit.
  const event = await c.ended; assert.equal(event.code,0,event.stderr);assert.equal(event.forced,false);assert.match(event.stdout,/Preview stopped \(stdin stop\)/);assert.match(event.stdout,/Packaged child waited:.*"code":0/);await assert.rejects(fetch(c.url));
  if(!event.stdout.includes('catalog reader waited: null')) {
    const value=JSON.parse(event.stdout.match(/catalog reader waited: (.+)/)[1]);
    const exits=Array.isArray(value)?value:[value];
    assert.deepEqual(exits.map(e=>e.code),Array.isArray(readerCode)?readerCode:[readerCode]);
    assert.ok(exits.every(e=>Number.isInteger(e.pid)&&e.signal===null));
  }
}
async function mode(c) { return JSON.parse((await(await fetch(c.url+'mode.mjs')).text()).slice(15,-1)); }

test('Explicit ZIP allowlist, independent .NET extraction and manifest identities at two locations', async()=>{
  assert.equal(sha(await fs.readFile(artifact.zipPath)),artifact.sha256);
  const expected = ['bin/obelisk.exe', ...runtimeFiles.map(n=>'preview/'+n), ...launcherFiles, 'LICENSE','THIRD-PARTY-NOTICES.txt','package-manifest.json'].sort();
  assert.deepEqual([...artifact.entries].sort(),expected);
  for(const pkg of packages){const r=await finite(ps,['-NoProfile','-File',path.join(scriptRoot,'extract.ps1'),artifact.zipPath,pkg]);assert.deepEqual(JSON.parse(r.stdout).sort(),expected);const manifest=JSON.parse(await fs.readFile(path.join(pkg,'package-manifest.json'),'utf8'));assert.equal(sha(await fs.readFile(path.join(pkg,'package-manifest.json'))),artifact.manifestSHA256);for(const f of manifest.files)assert.equal(sha(await fs.readFile(path.join(pkg,f.path))),f.sha256);const check=await launch(pkg,['check']);assert.match(check.stdout,/filesVerified/);}
});
test('Packaged binary is launcher-only: other modes refuse promptly, hold no port, write nothing; manifest records the build', async()=>{
  const net=await import('node:net');
  const bin=path.join(packages[0],'bin','obelisk.exe');
  const manifest=JSON.parse(await fs.readFile(path.join(packages[0],'package-manifest.json'),'utf8'));
  const port=await new Promise((resolve,reject)=>{const s=net.createServer();s.once('error',reject);s.listen(0,'127.0.0.1',()=>{const p=s.address().port;s.close(()=>resolve(p));});});
  const free=()=>new Promise(resolve=>{const s=net.createServer();s.once('error',()=>resolve(false));s.listen(port,'127.0.0.1',()=>s.close(()=>resolve(true)));});
  const home=path.join(evidence,'launcher-only-profile');
  // An existing legacy data directory is the case in which the full executable starts its server.
  for(const d of ['.obelisk','.mnemo']){await fs.mkdir(path.join(home,d),{recursive:true});await fs.writeFile(path.join(home,d,'config.json'),JSON.stringify({listen:'127.0.0.1:'+port}));}
  const profileBefore=await tree(home),data=path.join(evidence,'launcher-only-data');
  for(const args of [[],[''],['-listen','127.0.0.1:'+port],['-init-config'],['-data',data,'-init-config','-listen','127.0.0.1:'+port],['-data',path.join(home,'.obelisk')],['--help'],['-h'],['version'],['--version'],['serve'],['--gui-disposable-inventoryx'],['gui-catalog-readonly']]){
    const started=Date.now(),c=child(bin,args,{USERPROFILE:home,HOME:home});c.proc.stdin.end();
    const r=await c.ended;assert.equal(r.forced,false);assert.ok(Date.now()-started<5000,JSON.stringify(args)+' did not exit promptly');
    assert.equal(r.code,2,JSON.stringify(args)+r.stderr);assert.equal(r.stdout,'');
    assert.ok(r.stderr.includes('Obelisk '+manifest.version+': this build contains only --gui-disposable-inventory and --gui-catalog-readonly'),r.stderr);
    assert.equal(await free(),true,JSON.stringify(args)+' left port '+port+' in use');
  }
  await assert.rejects(fs.access(data));assert.deepEqual(await tree(home),profileBefore);
  assert.match(manifest.packageID,/^0\.9\.2-dev-comparison\.[0-9a-f]{12}-windows-amd64-local$/);
  assert.equal(manifest.version,'0.9.2-dev-comparison.'+manifest.sourceCommit.slice(0,12));assert.equal(manifest.packageID,manifest.version+'-windows-amd64-local');
  assert.match(manifest.binary,/launcher-only/);assert.ok(manifest.build.flags.includes('guionly'));assert.ok(manifest.build.flags.includes('-X main.appVersion='+manifest.version));assert.equal(manifest.build.GOPROXY,'off');
  assert.equal(manifest.build.toolchain,'go1.27.0');assert.equal(manifest.build.goVersion,'go version go1.27.0 windows/amd64');
  assert.match(manifest.sourceCommit,/^[0-9a-f]{40}$/);assert.equal(manifest.acceptedRuntimeImplementation,undefined);
  assert.equal(manifest.prerequisites.powershell,undefined);
});
test('Launch.cmd is plain ASCII with CRLF, replaces the PowerShell launcher, and passes the exit code through', async()=>{
  const bytes=await fs.readFile(path.join(packages[1],'Launch.cmd'));
  assert.ok(bytes.every(b=>b<0x80),'Launch.cmd must be ASCII');
  const text=bytes.toString('latin1');assert.equal(text.split('\r\n').length-1,text.split('\n').length-1,'Launch.cmd must use CRLF only');
  const commands=text.split('\r\n').filter(l=>l.trim()&&!/^rem\b/i.test(l.trim())).join('\n');
  assert.doesNotMatch(commands,/powershell|ExecutionPolicy/i);
  await assert.rejects(fs.access(path.join(packages[1],'Launch.ps1')));
  // The relocated package path contains spaces, non-ASCII, &, ', + and %.
  const ok=await launch(packages[1],['check']);assert.equal(JSON.parse(ok.stdout.trim().split('\n').at(-1)).root,packages[1]);
  await launch(packages[1],['no-such-action'],1);
});
test('Packaged executable reports the manifest version to the launcher, inventory results and the viewer', async()=>{
  const manifest=JSON.parse(await fs.readFile(path.join(packages[0],'package-manifest.json'),'utf8')),version=manifest.version;
  const check=await launch(packages[0],['check']);
  assert.ok(check.stdout.includes('Obelisk developer alpha '+version+' (package '+manifest.packageID+')'),check.stdout);
  assert.equal(JSON.parse(check.stdout.trim().split('\n').at(-1)).version,version);
  const ws=path.join(evidence,'version-workspace');await launch(packages[0],['generate',ws]);
  const inv=await launch(packages[0],['inventory',ws,'off.json']);
  const result=JSON.parse(inv.stdout.trim().split('\n').find(l=>l.startsWith('{"version"')));assert.equal(result.version,version);assert.equal(result.published,true);
  const c=await start(packages[0],['view',path.join(ws,'catalogs','off.json')]);
  try{for(let i=0;i<100&&!c.text().includes('Native catalog reader version: '+version);i++)await delay(20);assert.ok(c.text().includes('Native catalog reader version: '+version),c.text());assert.equal((await mode(c)).readerVersion,version);}finally{await stop(c);}
});
test('Default help is inert and missing Node gives a prerequisite failure', async()=>{
  const before=await tree(packages[0]);const r=await launch(packages[0],[]);assert.match(r.stdout,/Actions/);assert.deepEqual(await tree(packages[0]),before);
  const missing=await launch(packages[0],['check'],1,{PATH:path.join(process.env.SystemRoot,'System32')});assert.match(missing.stderr,/Node.js 24 x64 is required/);
});
test('Missing and wrong packaged adapter/assets refuse without alternate executable or demo fallback',async()=>{
  for(const [name,target,wrong]of [['missing-adapter','bin/obelisk.exe',false],['wrong-adapter','bin/obelisk.exe',true],['missing-assets','preview/index.html',false]]){
    const pkg=path.join(evidence,name);await finite(ps,['-NoProfile','-File',path.join(scriptRoot,'extract.ps1'),artifact.zipPath,pkg]);
    if(wrong)await fs.writeFile(path.join(pkg,target),'Synthetic wrong-file identity test; never executed');else await fs.unlink(path.join(pkg,target));
    const r=await launch(pkg,['static'],1);assert.match(r.stderr,/Missing package file|identity mismatch/);assert.doesNotMatch(r.stdout,/http:\/\//);
  }
});
test('Both relocations generate new fixtures and OFF/ON snapshots; reuse refuses and bytes persist',async()=>{
  for(let i=0;i<packages.length;i++){
    const pkg=packages[i],ws=workspaces[i];await launch(pkg,['generate',ws]);await launch(pkg,['generate',ws],1);const sourceBefore=await tree(path.join(ws,'source'));
    await launch(pkg,['inventory',ws,'off.json']);await launch(pkg,['inventory',ws,'on.json','--ignore-ds-store']);
    const off=JSON.parse(await fs.readFile(path.join(ws,'catalogs/off.json'),'utf8')),on=JSON.parse(await fs.readFile(path.join(ws,'catalogs/on.json'),'utf8'));
    assert.equal(off.files.length,10);assert.equal(on.files.length,8);assert.equal(JSON.parse(off.audit[0].detail).policy,'include-all');assert.equal(JSON.parse(on.audit[0].detail).excludedFiles,2);
    assert.ok(on.files.some(f=>f.rel_path==='directory/.DS_Store/keep.txt'));assert.ok(on.files.some(f=>f.rel_path==='.DS_Store.bak'));
    assert.equal(off.files.find(f=>f.rel_path==='same-one.txt').hash,off.files.find(f=>f.rel_path==='nested/same-two.txt').hash);
    const snapshots=await tree(path.join(ws,'catalogs'));const collision=await launch(pkg,['inventory',ws,'off.json'],1);assert.match(collision.stdout,/"published":false/);assert.deepEqual(await tree(path.join(ws,'catalogs')),snapshots);
    await launch(pkg,['inventory',ws,'off-2.json']);assert.deepEqual(await tree(path.join(ws,'source')),sourceBefore);
    await fs.writeFile(path.join(evidence,'source-'+i+'.json'),JSON.stringify(sourceBefore,null,2));
  }
});
test('Packaged viewers query exact names, expose scope and isolate files; split stop and reopen preserve inputs',async()=>{
  for(let i=0;i<packages.length;i++)for(const name of ['off','on','off']){
    const catalog=path.join(workspaces[i],'catalogs',name+'.json'),before=sha(await fs.readFile(catalog)),c=await start(packages[i],['view',catalog]);
    try{const m=await mode(c);assert.equal(m.ok,true);assert.equal(m.catalog.inventoryScope.excludedFiles,name==='on'?2:0);
      const q=await(await fetch(c.url+'catalog-query?'+new URLSearchParams({exact:"nested/O'Brien & +%# note.txt"}))).json();assert.equal(q.ok,true);assert.equal(q.ids.length,1);assert.equal(m.catalog.files.find(f=>f.id===q.ids[0]).path,"nested/O'Brien & +%# note.txt");
      for(const target of ['package-manifest.json','bin/obelisk.exe','../catalogs/off.json','api/scan','launcher.mjs','tutorial-workspace.json'])assert.equal((await fetch(c.url+target)).status,404);
    }finally{await stop(c);}assert.equal(sha(await fs.readFile(catalog)),before);
  }
});
test('Packaged native scope-key and encoding refusals, legacy UNKNOWN, exact control names and large IDs',async()=>{
  const raw=JSON.parse(await fs.readFile(path.join(workspaces[0],'catalogs/off.json'),'utf8'));const inputs=path.join(evidence,'protocol-inputs');await fs.mkdir(inputs);
  const bad=structuredClone(raw),detail=JSON.parse(bad.audit[0].detail);detail.Entries=detail.entries;delete detail.entries;bad.audit[0].detail=JSON.stringify(detail);
  const legacy=structuredClone(raw);legacy.audit=[];legacy.files=legacy.files.slice(0,2);legacy.files[0].id=701;legacy.files[0].rel_path='line\nname.txt';legacy.files[1].rel_path=String.raw`line\nname.txt`;
  const foreign=JSON.stringify(legacy).replace('"id":701','"id":9007199254740993');
  const cases=[['scope-refused',JSON.stringify(bad),false],['encoding-refused',foreign.replace('line\\nname.txt','line\\ud800name.txt'),false],['foreign',foreign,true]];
  for(const [name,bytes,ok]of cases){const file=path.join(inputs,name+'.json');await fs.writeFile(file,bytes,{flag:'wx'});const before=sha(await fs.readFile(file)),c=await start(packages[0],['view',file]);try{const m=await mode(c);assert.equal(m.ok,ok);if(ok){assert.equal(m.catalog.inventoryScope,null);const q=await(await fetch(c.url+'catalog-query?'+new URLSearchParams({exact:'line\nname.txt'}))).json();assert.deepEqual(q.ids,['9007199254740993']);}else{assert.equal(m.catalog,undefined);const q=await(await fetch(c.url+'catalog-query?text=')).json();assert.equal(q.ok,false);}}finally{await stop(c,ok?0:1);}assert.equal(sha(await fs.readFile(file)),before);}
  await fs.writeFile(path.join(evidence,'rehearsal-inputs.json'),JSON.stringify({packages,workspaces,inputs,cwd},null,2));
});
test('Actual packaged reader exit invalidates query state; static mode remains intentional',async()=>{
  const module=await import(pathToFileURL(path.join(packages[0],'preview/server.mjs'))),adapter=path.join(packages[0],'bin/obelisk.exe'),catalog=path.join(workspaces[0],'catalogs/off.json');
  let reader;const originalSpawn=childProcess.spawn;
  childProcess.spawn=(exe,args,options)=>{const c=originalSpawn(exe,args,options);if(exe===adapter)reader=c;return c;};syncBuiltinESMExports();
  let server;try{server=await module.startPreview(0,{catalog,adapter});}finally{childProcess.spawn=originalSpawn;syncBuiltinESMExports();}
  const url='http://127.0.0.1:'+server.address().port+'/';assert.ok(reader);
  try{const initial=JSON.parse((await(await fetch(url+'mode.mjs')).text()).slice(15,-1));assert.equal(initial.ok,true,'Fault probe requires a successfully loaded reader');const exited=new Promise(resolve=>reader.once('exit',resolve));assert.equal(reader.kill(),true);await exited;await delay(50);const m=JSON.parse((await(await fetch(url+'mode.mjs')).text()).slice(15,-1));assert.equal(m.ok,false);assert.equal(m.catalog,undefined);const response=await fetch(url+'catalog-query?text=');assert.equal(response.status,503);assert.equal((await response.json()).ok,false);}
  finally{await new Promise(resolve=>{server.close(resolve);server.closeAllConnections();});const exit=await server.catalogStopped;await fs.writeFile(path.join(evidence,'intentional-reader-fault.json'),JSON.stringify({reason:'Test-induced termination of captured task-owned reader; not normal shutdown',exit},null,2));await assert.rejects(fetch(url));}
  const c=await start(packages[1],['static']);try{assert.equal((await mode(c)).enabled,false);assert.match(c.text(),/INTENTIONAL STATIC DEMO/);assert.equal((await fetch(c.url+'catalog-query?text=')).status,404);}finally{await stop(c);}
});

test('Invalid pair launcher forms and missing inputs refuse without generation or demo fallback',async()=>{
  const pkg=packages[0],catalog=path.join(workspaces[0],'catalogs/off.json'),before=await tree(workspaces[0]);
  for(const args of [['view'],['view',catalog,catalog,catalog],['view',catalog,'relative.json'],['view',catalog,catalog],['view',catalog,path.join(evidence,'absent.json')],['static',catalog],['generate-pair'],['inventory',workspaces[0],'bad.json','--unknown']]) {
    const r=await launch(pkg,args,1);assert.doesNotMatch(r.stdout,/http:\/\/|Generated/);
  }
  assert.deepEqual(await tree(workspaces[0]),before);
});

test('Extracted ALPHA/BETA tutorial, independent classes, scope, reversal and two-reader stop/reopen',async()=>{
  const pairs=[];
  for(let i=0;i<packages.length;i++) {
    const pkg=packages[i],ws=path.join(evidence,i?'pair café O\'Brien & +%#':'pair-one');
    await launch(pkg,['generate-pair',ws]);const generated=await tree(ws);await launch(pkg,['generate-pair',ws],1);assert.deepEqual(await tree(ws),generated);
    const alpha=path.join(ws,'ALPHA'),beta=path.join(ws,'BETA');
    const sourceA=await tree(path.join(alpha,'source')),sourceB=await tree(path.join(beta,'source'));
    await launch(pkg,['inventory',alpha,'off.json']);await launch(pkg,['inventory',beta,'off.json']);await launch(pkg,['inventory',beta,'on.json','--ignore-ds-store']);
    const a=path.join(alpha,'catalogs/off.json'),b=path.join(beta,'catalogs/off.json'),on=path.join(beta,'catalogs/on.json');
    const catalogsBefore=[await tree(path.join(alpha,'catalogs')),await tree(path.join(beta,'catalogs'))];
    await launch(pkg,['inventory',alpha,'off.json'],1);
    for(const [left,right,policy]of [[a,b,'off'],[b,a,'reverse'],[a,on,'on'],[a,b,'reopen']]) {
      const c=await start(pkg,['view',left,right]);
      try {
        const m=await mode(c);assert.equal(m.ok,true);assert.equal(m.multi,true);assert.equal(m.snapshots.length,2);
        assert.equal(m.readerVersion,JSON.parse(await fs.readFile(path.join(pkg,'package-manifest.json'),'utf8')).version);
        for(const [j,file]of [left,right].entries())assert.equal(m.snapshots[j].catalog.digest,sha(await fs.readFile(file)));
        const all=await(await fetch(c.url+'catalog-query?text=&hash=&snapshot=all')).json();assert.equal(all.groups.length,2);assert.equal(all.groups[0].ids.length,11);assert.equal(all.groups[1].ids.length,policy==='on'?9:11);
        for(const reverse of [false,true]) {
          const [ref,other]=reverse?[...m.snapshots].reverse():m.snapshots;
          const response=await fetch(c.url+'catalog-compare?'+new URLSearchParams({reference:ref.handle,counterpart:other.handle,request:randomUUID()}));const result=await response.json();assert.equal(response.status,200);
          const expected=policy==='on'?{agreement:7,difference:1,inconclusive:0,referenceOnly:reverse?1:3,counterpartOnly:reverse?3:1}:{agreement:9,difference:1,inconclusive:0,referenceOnly:1,counterpartOnly:1};
          assert.deepEqual(result.counts,expected);assert.equal(result.totals.union,12);assert.equal(result.rows.find(r=>r.key==="nested/O'Brien & +%# note.txt").kind,'difference');
          assert.equal(result.rows.find(r=>r.key==='same-one.txt').kind,'agreement');assert.equal(result.rows.find(r=>r.key==='nested/same-two.txt').kind,'agreement');
          if(policy==='on')assert.match(result.rows.find(r=>r.key==='.DS_Store').scopeNote,/outside/);
        }
        for(const target of ['package-manifest.json','bin/obelisk.exe','launcher.mjs','api/scan','tutorial-fixtures.mjs'])assert.equal((await fetch(c.url+target)).status,404);
      } finally {await stop(c,[0,0]);}
    }
    assert.deepEqual(await tree(path.join(alpha,'catalogs')),catalogsBefore[0]);assert.deepEqual(await tree(path.join(beta,'catalogs')),catalogsBefore[1]);
    assert.deepEqual(await tree(path.join(alpha,'source')),sourceA);assert.deepEqual(await tree(path.join(beta,'source')),sourceB);
    pairs.push({workspace:ws,a,b,on});
  }
  const inputs=JSON.parse(await fs.readFile(path.join(evidence,'rehearsal-inputs.json'),'utf8'));inputs.pairs=pairs;
  await fs.writeFile(path.join(evidence,'rehearsal-inputs.json'),JSON.stringify(inputs,null,2));
});

test('Duplicate artifact and bad second reader refuse the complete pair, followed by clean reopen',async()=>{
  const inputs=JSON.parse(await fs.readFile(path.join(evidence,'rehearsal-inputs.json'),'utf8')),a=inputs.pairs[0].a,b=inputs.pairs[0].b;
  const duplicate=path.join(evidence,'protocol-inputs','renamed-copy.json');await fs.copyFile(a,duplicate);
  for(const [other,codes]of [[duplicate,[0,0]],[path.join(inputs.inputs,'scope-refused.json'),[0,1]]]) {
    const c=await start(packages[0],['view',a,other]);try {const m=await mode(c);assert.equal(m.ok,false);assert.equal(m.snapshots,undefined);const response=await fetch(c.url+'catalog-compare?reference=invalid');assert.equal(response.status,503);assert.equal((await response.json()).ok,false);}finally{await stop(c,codes);}
  }
  const c=await start(packages[0],['view',a,b]);try{assert.equal((await mode(c)).ok,true);}finally{await stop(c,[0,0]);}
});
