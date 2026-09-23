import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { startPreview } from './server.mjs';
import { validateMatches, validateSnapshots } from './catalog-protocol.mjs';

const root=process.env.OBELISK_GUI_MULTI_FIXTURES, adapter=process.env.OBELISK_GUI_TEST_ADAPTER;
const old=process.env.OBELISK_GUI_TEST_INPUTS, correction=process.env.OBELISK_GUI_CORRECTION_INPUTS, double=process.env.OBELISK_GUI_TEST_DOUBLE;
if(!root||!adapter||!old||!correction||!double)throw Error('Explicit task-owned fixtures/readers required');
const p=n=>path.join(root,n+'.json'), base=s=>'http://127.0.0.1:'+s.address().port;
const mode=async s=>JSON.parse((await(await fetch(base(s)+'/mode.mjs')).text()).slice(15,-1));
const start=(a='a',b='b-aged')=>startPreview(0,{catalogs:[p(a),p(b)],adapter});
async function close(s){const url=base(s);await new Promise(r=>{s.close(r);s.closeAllConnections()});const exits=await s.catalogStopped;assert.equal(exits.length,2);assert.ok(exits.every(x=>x.code!==null||x.signal!==null));await assert.rejects(fetch(url));return exits;}
const query=async(s,filter,q={})=>{const r=await fetch(base(s)+'/catalog-query?'+new URLSearchParams({snapshot:filter,...q}));return {status:r.status,body:await r.json()};};
async function identities(dir){const out=[];for(const e of await readdir(dir,{withFileTypes:true})){const name=path.join(dir,e.name);out.push([e.name,e.isDirectory()?await identities(name):createHash('sha256').update(await readFile(name)).digest('hex')]);}return out;}

test('Real pair queries bind overlapping IDs, reverse order and reopen without input writes',async t=>{
 const before=await identities(root), seen=[];
 for(const pair of [['a','b-aged'],['b-aged','a'],['a','b-aged']]) {
  const s=await start(...pair);try {
   const m=await mode(s);assert.equal(m.ok,true);validateSnapshots(m.snapshots);seen.push(m.snapshots.map(x=>x.catalog.digest).sort());
   const all=await query(s,'all');assert.equal(all.status,200);assert.equal(validateMatches(all.body,m.snapshots,'all').length,5);
   for(const snap of m.snapshots){const q=await query(s,snap.handle,{exact:'shared.txt'});assert.equal(q.status,200);const records=validateMatches(q.body,m.snapshots,snap.handle);assert.equal(records.length,1);assert.equal(records[0].snapshot,snap.handle);const file=snap.catalog.files.find(f=>f.id===records[0].id);assert.equal(file.path,'shared.txt');}
   const shared=m.snapshots.map(s=>s.catalog.files.find(f=>f.path==='shared.txt'));assert.notEqual(shared[0].hash,shared[1].hash);
   for(const q of ['snapshot=invalid','snapshot=all&snapshot=all','snapshot=all&path='+encodeURIComponent(p('a')),'text=shared'])assert.equal((await fetch(base(s)+'/catalog-query?'+q)).status,400);
   assert.equal(validateMatches((await query(s,'all',{exact:'absent-record'})).body,m.snapshots,'all').length,0);
  }finally{t.diagnostic(JSON.stringify(await close(s)));}
 }
 assert.deepEqual(seen[0],seen[1]);assert.deepEqual(seen[0],seen[2]);assert.deepEqual(await identities(root),before);
});

test('Duplicate inputs, byte-identical copies, invalid second and native refusal controls fail whole startup',async t=>{
 const cases=JSON.parse(await readFile(p('scope-cases'),'utf8'));
 const inputs=[p('a'),p('a-copy'),p('missing'),...['malformed','future','zero','directory'].map(n=>path.join(old,n+'.json')),...cases.filter(c=>!c.ok).map(c=>p(c.name))];
 const before=await identities(root);
 for(const input of inputs){const s=await startPreview(0,{catalogs:[p('a'),input],adapter});try{const m=await mode(s);assert.equal(m.enabled,true);assert.equal(m.ok,false,input);assert.equal(m.snapshots,undefined);assert.equal((await query(s,'all')).status,503);}finally{await close(s);}}
 await assert.rejects(startPreview(0,{catalogs:[p('a'),p('b'),p('zero')],adapter}));
 const oversized=await start('limit-a','limit-b');try{assert.equal((await mode(oversized)).ok,false);assert.match((await mode(oversized)).error,/aggregate limit/);assert.equal((await query(oversized,'all')).status,503);}finally{await close(oversized);}
 assert.deepEqual(await identities(root),before);t.diagnostic(`${inputs.length} refusal pairings; source inputs unchanged`);
});

test('Native OFF/ON/UNKNOWN, known zero, empty/excluded and recorded offset remain per snapshot',async()=>{
 for(const [a,b] of [['a','b-aged'],['unknown','zero'],['empty','excluded']]){const s=await start(a,b);try{
  const m=await mode(s);assert.equal(m.ok,true);const [x,y]=m.snapshots.map(s=>s.catalog);
  if(a==='a'){assert.equal(x.inventoryScope.policy,'include-all');assert.equal(y.inventoryScope.policy,'ignore-exact-ds-store');assert.equal(y.recordedAt,'2024-02-03T04:05:06-05:00');assert.notEqual(y.recordedAt,y.loadedAt);}
  if(a==='unknown'){assert.equal(x.inventoryScope,null);assert.equal(x.recordedAt,null);assert.equal(y.inventoryScope.excludedFiles,0);}
  if(a==='empty'){assert.equal(x.inventoryScope.entries,0);assert.equal(y.inventoryScope.excludedFiles,1);assert.equal(y.files.length,0);}
 }finally{await close(s);}}
});

test('Large IDs and control-character exact names use real native query transport',async()=>{
 for(const pair of [['large-a','large-b'],['controls','unknown']]){const s=await start(...pair);try{
  const m=await mode(s);assert.equal(m.ok,true);
  for(const snap of m.snapshots)for(const f of snap.catalog.files){const q=await query(s,snap.handle,{exact:f.path});assert.equal(q.status,200);assert.ok(validateMatches(q.body,m.snapshots,snap.handle).some(r=>r.id===f.id));}
  if(pair[0]==='large-a')assert.ok(m.snapshots.every(s=>s.catalog.files.some(f=>f.id==='9007199254740993')));
 }finally{await close(s);}}
});

test('Malformed, unknown, duplicate, numeric and cross-filter identities cannot resolve another record',async()=>{
 const s=await start();try{const m=await mode(s),[a,b]=m.snapshots;
  for(const groups of [[{snapshot:a.handle,ids:['999']}],[{snapshot:a.handle,ids:[1]}],[{snapshot:b.handle,ids:['1']}],[{snapshot:a.handle,ids:['1','1']}],[{snapshot:'f'.repeat(32),ids:['1']}],[]])assert.throws(()=>validateMatches({ok:true,groups},m.snapshots,a.handle));
  assert.throws(()=>validateMatches({ok:true,groups:[{snapshot:a.handle,ids:[]}]},m.snapshots,'all'));
  const huge=structuredClone(m.snapshots);for(const snap of huge){snap.catalog.inventoryScope=null;snap.catalog.files=Array.from({length:501},(_,i)=>({...snap.catalog.files[0],id:String(i+1)}));}assert.throws(()=>validateSnapshots(huge));
 }finally{await close(s);}
});

test('Fault-emitter second reader failure invalidates All; pending shutdown is bounded',async t=>{
 // Distinct projection fixture directories avoid duplicate digest masking the fault.
 const faultRoot=process.env.OBELISK_GUI_MULTI_FAULTS;
 for(const name of ['query-failed','unknown-id','late']){
  const s=await startPreview(0,{catalogs:[path.join(correction,'late.json'),path.join(faultRoot,name+'.json')],adapter:double});
  const m=await mode(s);assert.equal(m.ok,true);
  if(name==='late'){
   const pending=query(s,'all',{text:'delay'}).catch(()=>({closed:true}));await new Promise(r=>setTimeout(r,40));
   const begin=Date.now();const exits=await close(s);await pending;assert.ok(Date.now()-begin<2500);t.diagnostic(JSON.stringify({kind:'fault emitter; forced fallback may apply',exits}));
  }else{try{assert.equal((await query(s,'all',{hash:'bbbb'})).status,503);assert.equal((await mode(s)).ok,false);}finally{await close(s);}}
 }
});

test('Pair CLI split stop waits both readers with stdin open and listener closed',async t=>{
 const args=[fileURLToPath(new URL('./server.mjs',import.meta.url)),'0','--catalog',p('a'),'--catalog',p('b-aged'),'--adapter',adapter];
 const child=spawn(process.execPath,args,{windowsHide:true});const done=once(child,'exit');let out='';child.stdout.on('data',b=>out+=b);child.stderr.on('data',b=>out+=b);
 const timer=setTimeout(()=>child.kill(),15000);
 try{for(let i=0;i<200&&!out.includes('http://');i++)await new Promise(r=>setTimeout(r,20));const url=out.match(/http:\/\/127\.0\.0\.1:\d+/)?.[0];assert.ok(url);
  child.stdin.write('st');await new Promise(r=>setTimeout(r,40));assert.equal((await fetch(url)).status,200);child.stdin.write('op\n');assert.equal((await done)[0],0);assert.equal((out.match(/"code":0/g)||[]).length,2);await assert.rejects(fetch(url));t.diagnostic(JSON.stringify({pid:child.pid,args,out,stdinHeldOpen:true}));
 }finally{clearTimeout(timer);child.stdin.destroy();if(child.exitCode===null&&child.signalCode===null){child.kill();await done;}}
});
