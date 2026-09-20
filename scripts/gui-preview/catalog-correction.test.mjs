import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { startPreview } from './server.mjs';
import { exactDecimal, validateCatalog, validateIDs } from './catalog-protocol.mjs';

const inputs=process.env.OBELISK_GUI_TEST_INPUTS, fixtures=process.env.OBELISK_GUI_CORRECTION_INPUTS;
const adapter=process.env.OBELISK_GUI_TEST_ADAPTER, double=process.env.OBELISK_GUI_TEST_DOUBLE;
if(!inputs||!fixtures||!adapter||!double)throw Error('Set explicit correction inputs, adapter and test double');
const pause=ms=>new Promise(r=>setTimeout(r,ms));
const base=s=>`http://127.0.0.1:${s.address().port}`;
const mode=async s=>JSON.parse((await(await fetch(base(s)+'/mode.mjs')).text()).slice(15,-1));
async function close(s){const url=base(s);await new Promise(r=>{s.close(r);s.closeAllConnections();});const exit=await s.catalogStopped;await assert.rejects(fetch(url));return exit;}
async function waitFor(fn){for(let n=0;n<150;n++){if(await fn())return;await pause(10);}throw Error('Condition did not settle');}
const start=(name,reader=double)=>startPreview(0,{catalog:path.join(fixtures,name+'.json'),adapter:reader});

test('Exact decimal protocol rejects coercion, malformed, out-of-range and unknown identities', async()=>{
  const s=await start('exact',adapter);
  try {
    const data=(await mode(s)).catalog;validateCatalog(data);
    for(const id of ['1','9007199254740991','9007199254740992','9007199254740993','9223372036854775807'])assert.ok(exactDecimal(id,data.idMax));
    for(const id of [1,9007199254740992,'','0','-1','01','+1','1.0','1e0',' 1','9223372036854775808',null]) {
      assert.equal(exactDecimal(id,data.idMax),false,String(id));assert.throws(()=>validateIDs([id],data));
    }
    assert.throws(()=>validateIDs(['999'],data));assert.throws(()=>validateIDs(['1','1'],data));assert.throws(()=>validateCatalog({schema:8}));
    const bad=structuredClone(data);bad.files[1].id=bad.files[0].id;assert.throws(()=>validateCatalog(bad));
    const numeric=structuredClone(data);numeric.files[0].id=1;assert.throws(()=>validateCatalog(numeric));
  }finally{await close(s);}
});
test('Native exact identities and size evidence survive transport and reversed ordering',async t=>{
  const ids=['1','9007199254740991','9007199254740992','9007199254740993','9223372036854775807'];
  for(const name of ['exact','exact-reversed']) {
    const s=await start(name,adapter);
    try {
      const data=(await mode(s)).catalog;assert.deepEqual(data.files.map(f=>f.id),name==='exact'?ids:[...ids].reverse());
      for(const [i,id] of ids.entries()) {
        const hash=String.fromCharCode(97+i).repeat(64);
        const q=await(await fetch(base(s)+'/catalog-query?hash='+hash)).json();assert.deepEqual(q.ids,[id]);
        const f=new Map(data.files.map(f=>[f.id,f])).get(q.ids[0]);assert.equal(f.hash,hash);assert.equal(f.bytes,id);
        assert.equal(f.sourceFolder,'/synthetic-never-open/EXACT/'+id);assert.equal(f.copies[0].id,'chunk-'+id+'-copy-0');
      }
      assert.deepEqual((await(await fetch(base(s)+'/catalog-query?text=no-match')).json()).ids,[]);
    }finally{t.diagnostic(JSON.stringify(await close(s)));}
  }
});
test('Initial and bootstrap-then-exit failures invalidate the real server mode without fallback',async t=>{
  for(const name of ['failed-exit','shape','partial','timeout','numeric-startup']) {
    const s=await start(name);
    try {
      await waitFor(async()=>!(await mode(s)).ok);const m=await mode(s);assert.equal(m.enabled,true);assert.equal(m.catalog,undefined);
      assert.equal((await fetch(base(s)+'/catalog-query')).status,503);
    }finally{t.diagnostic(name+': '+JSON.stringify(await close(s)));}
  }
});
test('Query failures and invalid exact identities settle pending work and invalidate mode',async t=>{
  for(const name of ['query-failed','invalid-id','unknown-id','duplicate-id','out-of-range','negative-id','zero-id','leading-zero','exponent-id']) {
    const s=await start(name);
    try {
      assert.equal((await mode(s)).ok,true);assert.deepEqual((await(await fetch(base(s)+'/catalog-query')).json()).ids,['1','2']);
      const q=await fetch(base(s)+'/catalog-query?hash=bbbb');assert.equal(q.status,503);assert.equal((await q.json()).ok,false);
      assert.equal((await mode(s)).ok,false);assert.equal((await mode(s)).catalog,undefined);
      assert.equal((await fetch(base(s)+'/catalog-query')).status,503);
    }finally{t.diagnostic(name+': '+JSON.stringify(await close(s)));}
  }
  const s=await startPreview(0,{catalog:path.join(inputs,'alpha.json'),adapter});
  try {assert.equal((await fetch(base(s)+'/catalog-query?hash=%0A')).status,503);assert.equal((await mode(s)).ok,false);}
  finally{t.diagnostic('real native failure: '+JSON.stringify(await close(s)));}
  for(const label of ['alpha','beta','alpha']) {
    const s=await startPreview(0,{catalog:path.join(inputs,label+'.json'),adapter});
    try {assert.equal((await mode(s)).ok,true);assert.deepEqual((await(await fetch(base(s)+'/catalog-query?hash=bbbb')).json()).ids,['2']);}
    finally{assert.equal((await close(s)).code,0);}
  }
});
test('Shared static/catalog CLI consumes split lines while stdin remains open and waits for owned reader',async t=>{
  const cases=[['stop\n'],['st','op\n'],['s','t','o','p','\n'],['stop\r','\n'],['notstop\nstoplater\nunknown\nstop\nstop\n']];
  for(const catalog of [false,true])for(const chunks of cases) {
    const args=[fileURLToPath(new URL('./server.mjs',import.meta.url)),'0'];if(catalog)args.push('--catalog',path.join(inputs,'alpha.json'),'--adapter',adapter);
    const child=spawn(process.execPath,args,{windowsHide:true});const exited=once(child,'exit');let stdout='',stderr='';
    child.stdout.on('data',b=>stdout+=b);child.stderr.on('data',b=>stderr+=b);
    const deadline=setTimeout(()=>child.stdin.end('\nstop\n'),8000);const started=Date.now();
    try {
      await waitFor(()=>stdout.includes('http://'));const url=stdout.match(/http:\/\/127\.0\.0\.1:\d+\//)[0];
      for(let i=0;i<chunks.length;i++){
        await new Promise((resolve,reject)=>child.stdin.write(chunks[i],err=>err?reject(err):resolve()));
        // Coordinate incomplete prefixes with a live HTTP request; exact chunk
        // delivery to the parser is independently controlled in its unit suite.
        if(i<chunks.length-1)assert.equal((await fetch(url)).status,200);
      }
      const [code,signal]=await exited;assert.equal(code,0,stderr);assert.equal(signal,null);assert.equal(child.stdin.writableEnded,false,'EOF hid a broken parser');
      assert.match(stdout,/Preview stopped \(stdin stop\)/);assert.equal((stdout.match(/Preview stopped/g)||[]).length,1);
      if(catalog)assert.match(stdout,/"code":0,"signal":null/);
      await assert.rejects(fetch(url));t.diagnostic(JSON.stringify({catalog,chunks,pid:child.pid,elapsed:Date.now()-started,code,stdout}));
    }finally{clearTimeout(deadline);if(child.exitCode===null&&child.signalCode===null){child.stdin.end('\nstop\n');await exited;}child.stdin.destroy();}
  }
});
test('Final partial stop at EOF uses the same path; native reader alone terminates on EOF',async t=>{
  const child=spawn(process.execPath,[fileURLToPath(new URL('./server.mjs',import.meta.url)),'0'],{windowsHide:true});const exited=once(child,'exit');let output='';child.stdout.on('data',b=>output+=b);
  await waitFor(()=>output.includes('http://'));child.stdin.end(' stop\r');assert.equal((await exited)[0],0);assert.match(output,/stdin stop/);
  const native=spawn(adapter,['--gui-catalog-readonly',path.join(inputs,'alpha.json')],{windowsHide:true});const done=once(native,'exit');let data='';native.stdout.on('data',b=>data+=b);native.stderr.on('data',()=>{});
  await waitFor(()=>data.includes('\n'));native.stdin.end();assert.equal((await done)[0],0);t.diagnostic('native EOF exit 0; pipe-only test, not an interactive console Ctrl+C test');
});
test('Bare EOF preserves the listener until the real SIGINT handler is dispatched by test coordination',async t=>{
  for(const catalog of [false,true]) {
    const args=[fileURLToPath(new URL('./testdata/signal-launcher.mjs',import.meta.url)),'0'];if(catalog)args.push('--catalog',path.join(inputs,'alpha.json'),'--adapter',adapter);
    const child=spawn(process.execPath,args,{windowsHide:true,stdio:['pipe','pipe','pipe','ipc']});const exited=once(child,'exit');const eof=once(child,'message');let output='';child.stdout.on('data',b=>output+=b);
    await waitFor(()=>output.includes('http://'));const url=output.match(/http:\/\/127\.0\.0\.1:\d+\//)[0];
    child.stdin.end();assert.equal((await eof)[0].eofObserved,true);assert.equal((await fetch(url)).status,200);
    child.send('interrupt');assert.equal((await exited)[0],0);assert.match(output,/Preview stopped \(SIGINT\)/);await assert.rejects(fetch(url));
    if(catalog)assert.match(output,/"code":0,"signal":null/);
    t.diagnostic(JSON.stringify({catalog,pid:child.pid,output,kind:'simulated process SIGINT event after observed pipe EOF; not interactive console'}));
  }
});
test('Shutdown settles an in-flight request and bounds a delayed owned reader',async t=>{
  const s=await start('late');
  const request=fetch(base(s)+'/catalog-query?text=delay').then(r=>r.json(),()=>({connectionClosed:true}));
  await pause(50);
  // The busy response establishes that the first operation actually occupies
  // the reader slot; elapsed time alone is not the evidence of pending work.
  assert.equal((await fetch(base(s)+'/catalog-query?hash=bbbb')).status,503);
  const started=Date.now();const exit=await close(s);const settled=await request;
  assert.ok(settled.connectionClosed||settled.ok===false);
  assert.ok(Date.now()-started<2000);assert.ok(exit.code!==null||exit.signal!==null);
  t.diagnostic(JSON.stringify({exit,settled,elapsed:Date.now()-started,scope:'delayed synthetic reader; bounded termination fallback is not native graceful EOF'}));
});
