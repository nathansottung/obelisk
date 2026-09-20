import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import http from 'node:http';
import { startPreview } from './server.mjs';

const root = process.env.OBELISK_GUI_TEST_INPUTS;
const adapter = process.env.OBELISK_GUI_TEST_ADAPTER;
if (!root || !adapter) throw new Error('Set explicit synthetic OBELISK_GUI_TEST_INPUTS and OBELISK_GUI_TEST_ADAPTER');
const inventory = async () => {
  const out=[];
  for(const e of await readdir(root,{withFileTypes:true})) {
    const p=path.join(root,e.name);
    out.push({name:e.name,directory:e.isDirectory(),value:e.isDirectory()?await readdir(p):createHash('sha256').update(await readFile(p)).digest('hex')});
  }
  return out.sort((a,b)=>a.name.localeCompare(b.name));
};
const mode = async base => JSON.parse((await (await fetch(base+'/mode.mjs')).text()).slice('export default '.length,-1));
const close = async server => {await new Promise(r=>{server.close(r);server.closeAllConnections();});const exit=await server.catalogStopped;assert.ok(exit && (exit.code!==null||exit.signal!==null));return exit;};

test('Two native snapshots and restart have distinct persisted data, native queries and no input writes',async t=>{
  const before=await inventory();const digests=[];
  for(const [name,label] of [['alpha','ALPHA'],['beta','BETA'],['alpha','ALPHA']]) {
    const s=await startPreview(0,{catalog:path.join(root,name+'.json'),adapter});
    try {
      assert.equal(s.address().address,'127.0.0.1');const base=`http://127.0.0.1:${s.address().port}`;
      const m=await mode(base);assert.equal(m.ok,true);assert.equal(m.catalog.files.length,2);assert.ok(m.catalog.collections[0].name.startsWith(label));digests.push(m.catalog.digest);
      for(const [query,ids] of [['text='+label,['1','2']],['hash=aaaa',['1']],['hash=bbbb',['2']],['text=absent',[]]])assert.deepEqual((await (await fetch(base+'/catalog-query?'+query)).json()).ids,ids);
      const concurrent=await Promise.all(['aaaa','bbbb'].map(async(hash,i)=>{const r=await fetch(base+'/catalog-query?hash='+hash);const data=await r.json();if(r.status===200)assert.deepEqual(data.ids,[String(i+1)]);else assert.equal(r.status,503);return r.status;}));
      assert.ok(concurrent.includes(200));
      assert.deepEqual((await (await fetch(base+'/catalog-query?hash=bbbb')).json()).ids,['2']);
      assert.equal(m.catalog.files[0].copies.length,2);assert.equal(m.catalog.files[1].copies.length,0);assert.ok(m.catalog.files[1].sourceFolder.includes(label+'/two'));
    } finally {t.diagnostic(JSON.stringify(await close(s)));}
  }
  assert.notEqual(digests[0],digests[1]);assert.equal(digests[0],digests[2]);assert.deepEqual(await inventory(),before);
});

test('Refused inputs never become static or valid-empty fallback and create no state',async t=>{
  const before=await inventory();
  for(const name of ['empty','malformed','zero','missing','future','directory']) {
    const s=await startPreview(0,{catalog:path.join(root,name+'.json'),adapter});
    try {
      const base=`http://127.0.0.1:${s.address().port}`;const m=await mode(base);assert.equal(m.enabled,true);assert.equal(m.ok,name==='empty');
      if(name==='empty')assert.deepEqual(m.catalog.files,[]);else {assert.equal(m.catalog,undefined);assert.equal((await fetch(base+'/catalog-query')).status,503);}
    } finally {t.diagnostic(name+': '+JSON.stringify(await close(s)));}
  }
  assert.deepEqual(await inventory(),before);
});

test('Catalog responses expose only the projection; routes and methods cannot select filesystem inputs',async t=>{
  const s=await startPreview(0,{catalog:path.join(root,'alpha.json'),adapter});
  try {
    const base=`http://127.0.0.1:${s.address().port}`;
    for(const p of ['/catalog.json','/alpha.json','/catalog-adapter.mjs','/catalog.test.mjs','/gui_catalog.go','/docs/Obelisk.fig','/docs/OBELISK_Readable_Design_References.zip','/Untitled.pdf','/../inputs/alpha.json','/?path=alpha.json','/api/jobs']) {const r=await fetch(base+p);assert.equal(r.status,404,p);await r.text();}
    for(const q of ['path=alpha.json','text=a&text=b','hash='+('a'.repeat(65))]) {const r=await fetch(base+'/catalog-query?'+q);assert.equal(r.status,400);await r.text();}
    for(const method of ['POST','PUT','PATCH','DELETE','OPTIONS']) {const r=await fetch(base+'/catalog-query',{method});assert.equal(r.status,405);await r.text();}
    const hostStatus=await new Promise((resolve,reject)=>{const req=http.get(base+'/catalog-query',{headers:{Host:'foreign.invalid'}},res=>{res.resume();resolve(res.statusCode);});req.on('error',reject);});assert.equal(hostStatus,403);
    const m=await mode(base);assert.equal(m.catalog.keys,undefined);assert.equal(m.catalog.chunks,undefined);assert.equal(m.catalog.next_id,undefined);
  } finally {t.diagnostic(JSON.stringify(await close(s)));}
});

test('Documented catalog CLI starts fresh, serves selected persisted input and stops/waits',async t=>{
  const child=spawn(process.execPath,[fileURLToPath(new URL('./server.mjs',import.meta.url)),'0','--catalog',path.join(root,'beta.json'),'--adapter',adapter],{windowsHide:true});
  const stopped=once(child,'exit');const timer=setTimeout(()=>child.kill(),15000);
  try {
    const base=await new Promise((resolve,reject)=>{let text='';child.on('error',reject);child.on('exit',()=>reject(new Error('early CLI exit')));child.stdout.on('data',b=>{text+=b;const match=text.match(/http:\/\/127\.0\.0\.1:\d+/);if(match)resolve(match[0]);});});
    assert.ok((await mode(base)).catalog.collections[0].name.startsWith('BETA'));t.diagnostic(`CLI PID ${child.pid}, ${base}`);
    // Windows process.kill is abrupt; exercise the launcher's explicit graceful stop.
    child.stdin.end('stop\n');const [code,signal]=await stopped;assert.equal(code,0);t.diagnostic(`CLI waited: ${code}/${signal}`);
  } finally {clearTimeout(timer);if(child.exitCode===null&&child.signalCode===null){child.kill();await stopped;}}
});
