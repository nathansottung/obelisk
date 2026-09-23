import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import path from 'node:path';
import {randomUUID,createHash} from 'node:crypto';
import {compareRecorded,classifyPair,comparisonFrame,validateComparison} from './recorded-comparison.mjs';
import {startPreview} from './server.mjs';
const root=process.env.OBELISK_GUI_COMPARISON_FIXTURES,adapter=process.env.OBELISK_GUI_TEST_ADAPTER,multi=process.env.OBELISK_GUI_MULTI_FIXTURES,faults=process.env.OBELISK_GUI_COMPARISON_FAULTS,double=process.env.OBELISK_GUI_COMPARISON_DOUBLE;
if(!root||!adapter||!multi||!faults||!double)throw Error('Explicit disposable comparison fixtures/readers required');
const full=c=>c.repeat(64),e=(bytes='4',hash=full('a'),algorithm='sha256')=>({bytes,hash,algorithm});
const mode=async s=>JSON.parse((await(await fetch(url(s)+'/mode.mjs')).text()).slice(15,-1));
const url=s=>`http://127.0.0.1:${s.address().port}`;
const stop=async(s,t)=>{const base=url(s);await new Promise(r=>{s.close(r);s.closeAllConnections()});const exits=await s.catalogStopped;assert.ok(exits.every(e=>e.code!==null||e.signal));await assert.rejects(fetch(base));t.diagnostic(JSON.stringify(exits));};
const compare=async(s,m,reverse=false)=>{const [a,b]=reverse?[...m.snapshots].reverse():m.snapshots;const response=await fetch(url(s)+'/catalog-compare?'+new URLSearchParams({reference:a.handle,counterpart:b.handle,request:randomUUID()}));return {status:response.status,value:await response.json()};};

test('Independent classification table: precedence, exact sizes, typed full digests and insufficient evidence',()=>{
 const cases=[['agreement',e(),e()],['difference',e(),e('4',full('b'))],['difference',e(),e('5',full('b'))],['inconclusive',e(),e('5')],['inconclusive',e('4',''),e('4','')],['inconclusive',e(),e('4',full('a'),'sha512')],['inconclusive',e('4','abcd'),e('4','abcd')],['difference',e('9007199254740992',''),e('9007199254740993','')],['inconclusive',e(9007199254740992),e()],['agreement',e(),e('4',full('A'))],['referenceOnly',e(),null],['counterpartOnly',null,e()]];
 for(const [kind,a,b]of cases)assert.equal(classifyPair(a,b).kind,kind);
 // Unsupported digest type is a pure-boundary control, not accepted native SHA-512 support.
});

test('Real native enumeration joins complete exact keys and independent expected classes, both directions and reopen',async t=>{
 const files=['a','b'].map(n=>path.join(root,n+'.json'));const before=await Promise.all(files.map(f=>readFile(f)));
 for(const reverseLaunch of [false,true,false]){
  const s=await startPreview(0,{catalogs:reverseLaunch?[...files].reverse():files,adapter});
  try{const m=await mode(s);assert.equal(m.ok,true);for(const reverse of [false,true]){
   const {status,value:v}=await compare(s,m,reverse);assert.equal(status,200);assert.equal(v.complete,true);assert.deepEqual(v.counts,{agreement:7,difference:3,inconclusive:2,referenceOnly:4,counterpartOnly:4});assert.deepEqual(v.totals,{reference:16,counterpart:16,union:20});
   for(const [key,kind]of [['agreement','agreement'],['different','difference'],['size','difference'],['unknown','inconclusive'],['contradiction','inconclusive'],['large','difference'],['line\nname','agreement'],['line\rname','agreement'],['line\r\nname','agreement'],['line\\nname','agreement']])assert.equal(v.rows.find(r=>r.key===key).kind,kind);
   const forward=reverseLaunch===reverse;assert.equal(v.rows.find(r=>r.key==='only-reference').kind,forward?'referenceOnly':'counterpartOnly');assert.equal(v.rows.find(r=>r.key==='large').referenceId,forward?'9007199254740993':'9007199254740994');
   assert.equal(new Set(v.rows.map(r=>r.key)).size,20);assert.equal(validateComparison(v,m.snapshots,v.reference,v.counterpart,v.request),v);
   for(const mutate of [v=>v.reference='unknown',v=>v.rows[0].referenceId='999',v=>v.rows.pop(),v=>v.request=randomUUID()]){const bad=structuredClone(v);mutate(bad);assert.throws(()=>validateComparison(bad,m.snapshots,v.reference,v.counterpart,v.request));}
  }}finally{await stop(s,t)}
 }
 for(let i=0;i<files.length;i++)assert.deepEqual(await readFile(files[i]),before[i]);
});

test('Ambiguous or incompatible frames refuse comparison while browsing remains usable',async t=>{
 for(const name of ['duplicate','outside','multi-root']){const s=await startPreview(0,{catalogs:[path.join(root,name+'.json'),path.join(root,'b.json')],adapter});try{const m=await mode(s);assert.equal(m.ok,true);const r=await compare(s,m);assert.equal(r.status,422);assert.equal(r.value.ok,false);assert.equal((await fetch(url(s)+'/catalog-query?text=&hash=&snapshot=all')).status,200);}finally{await stop(s,t)}}
});

test('Scope and time qualify one-sided .DS_Store, empty, excluded and historical UNKNOWN sets',async t=>{
 const off=await startPreview(0,{catalogs:['off-0','off-1'].map(n=>path.join(root,n+'.json')),adapter});try{const m=await mode(off);assert.ok(m.snapshots.every(s=>s.catalog.inventoryScope.policy==='include-all'));assert.equal((await compare(off,m)).value.counts.difference,1)}finally{await stop(off,t)}
 for(const [a,b]of [['a','b-aged'],['a','unknown'],['zero','b'],['empty','excluded'],['a','zero']]){
  const s=await startPreview(0,{catalogs:[path.join(multi,a+'.json'),path.join(multi,b+'.json')],adapter});try{const m=await mode(s),r=await compare(s,m);assert.equal(r.status,200);assert.equal(r.value.totals.reference,m.snapshots[0].catalog.files.length);if(b==='b-aged'){assert.equal(m.snapshots[1].catalog.recordedAt,'2024-02-03T04:05:06-05:00');assert.match(r.value.rows.find(r=>r.key==='.DS_Store').scopeNote,/outside/)}if(a==='empty')assert.equal(r.value.totals.union,0);if(b==='unknown')assert.equal(m.snapshots[1].catalog.inventoryScope,null);}finally{await stop(s,t)}
 }
});

test('Enumeration includes retired rows, refuses stale/partial/failed readers and stays bounded',async t=>{
 const s=await startPreview(0,{catalogs:[path.join(root,'retired.json'),path.join(root,'b.json')],adapter});try{const m=await mode(s);assert.equal((await compare(s,m)).value.totals.reference,16)}finally{await stop(s,t)}
 for(const name of ['partial','truncated','unknown','dies','timeout']){const s=await startPreview(0,{catalogs:[path.join(faults,'A','good.json'),path.join(faults,'B',name+'.json')],adapter:double});try{const m=await mode(s);assert.equal(m.ok,true);const r=await compare(s,m);assert.equal(r.status,503);assert.equal(r.value.ok,false);assert.equal((await mode(s)).ok,false);}finally{await stop(s,t)}}
});

test('Comparison route rejects selectors/parameters and unsupported modes; aggregate caps remain',async t=>{
 const s=await startPreview(0,{catalogs:[path.join(root,'a.json'),path.join(root,'b.json')],adapter});try{const m=await mode(s),p=new URLSearchParams({reference:m.snapshots[0].handle,counterpart:m.snapshots[1].handle,request:randomUUID()});for(const suffix of ['&path=C:/private','&reference=unknown','&request=bad'])assert.equal((await fetch(url(s)+'/catalog-compare?'+p+suffix)).status,400);assert.equal((await fetch(url(s)+'/catalog-compare?'+p,{method:'POST'})).status,405);p.set('reference','unknown');assert.equal((await fetch(url(s)+'/catalog-compare?'+p)).status,400);p.set('reference',m.snapshots[1].handle);assert.equal((await fetch(url(s)+'/catalog-compare?'+p)).status,400);}finally{await stop(s,t)}
 const capped=await startPreview(0,{catalogs:['limit-a','limit-b'].map(n=>path.join(multi,n+'.json')),adapter});try{assert.equal((await mode(capped)).ok,false)}finally{await stop(capped,t)}
 const oversized=await startPreview(0,{catalogs:[0,1].map(i=>path.join(faults,'big'+i,'good.json')),adapter:double});try{const m=await mode(oversized);assert.equal(m.ok,true);const r=await compare(oversized,m);assert.equal(r.status,422);assert.match(r.value.error,/8 MiB/)}finally{await stop(oversized,t)}
});

test('Pure frame refuses unsupported exact keys and resource overflow without normalization',async t=>{
 const s=await startPreview(0,{catalogs:[path.join(root,'a.json'),path.join(root,'b.json')],adapter});try{const m=await mode(s);for(const key of ['', '/absolute', '\\unc', 'C:/drive','a//b','a/./b','a/../b','nul\0name']){const bad=structuredClone(m.snapshots[0]);bad.catalog.files[0].path=key;assert.throws(()=>comparisonFrame(bad))}
 const big=structuredClone(m.snapshots);for(const [j,s]of big.entries()){s.catalog.files=Array.from({length:500},(_,i)=>({...s.catalog.files[0],id:String(i+1),path:j+'/'+i+'/'+'\u0001'.repeat(4000)}));s.catalog.comparisonFrame.recordCount=500;}assert.throws(()=>compareRecorded(big,big[0].handle,big[1].handle,randomUUID()),/8 MiB/);
 }finally{await stop(s,t)}
});
