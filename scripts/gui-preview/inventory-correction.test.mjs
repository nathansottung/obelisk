import test from 'node:test';
import assert from 'node:assert/strict';
import { parseExactName, displayName, decodeCatalogResponse } from './catalog-names.mjs';
import { validateCatalog } from './catalog-protocol.mjs';
import { startPreview } from './server.mjs';
import path from 'node:path';
import { readFile } from 'node:fs/promises';
const root=process.env.OBELISK_GUI_INVENTORY_FIX_INPUTS,adapter=process.env.OBELISK_GUI_TEST_ADAPTER;
if(!root||!adapter)throw Error('Provide explicit correction fixtures and fresh adapter');
const url=s=>'http://127.0.0.1:'+s.address().port;
async function close(s){const old=url(s);await new Promise(r=>{s.close(r);s.closeAllConnections()});const exit=await s.catalogStopped;assert.equal(exit.code,0);await assert.rejects(fetch(old));}
test('Explicit exact-name representation distinguishes controls and literal escapes',()=>{
  for(const value of ['line\nname','line\rname','line\r\nname',String.raw`line\nname`,'\ttab\t',' leading ','\ufffd','\ud83d\ude80',String.raw`C:\folder\new.txt`])assert.equal(parseExactName(JSON.stringify(value)),value);
  for(const entry of ['"\\ud800"','"\\udfff"','unquoted','null','12','"bad\\q"','"'+String.fromCharCode(0xd800)+'"'])assert.throws(()=>parseExactName(entry));
  assert.equal(parseExactName('"\\\\uD800"'),String.raw`\uD800`);
  assert.equal(displayName('line\nname'),'JSON name: "line\\nname"');assert.equal(displayName(String.raw`line\nname`),String.raw`line\nname`);
});
test('Protocol bytes refuse UTF-8 replacement; scalar validation precedes display',async()=>{
  const s=await startPreview(0,{catalog:path.join(root,'foreign.json'),adapter});
  try{
    const mode=JSON.parse((await(await fetch(url(s)+'/mode.mjs')).text()).slice(15,-1));
    for(const index of [0,mode.catalog.files.length-1]){
      const data=structuredClone(mode.catalog);data.files[index].path='ENCODING_PROBE';const raw=Buffer.from(JSON.stringify(data));
      const at=raw.indexOf('ENCODING_PROBE');assert.ok(at>=0);
      for(const invalid of [Buffer.from([255]),Buffer.from([0xc3]),Buffer.from([0xed,0xa0,0x80])])assert.throws(()=>decodeCatalogResponse(Buffer.concat([raw.subarray(0,at),invalid,raw.subarray(at+14)])));
      const surrogate=Buffer.concat([raw.subarray(0,at),Buffer.from('\\ud800'),raw.subarray(at+14)]);
      assert.throws(()=>validateCatalog(decodeCatalogResponse(surrogate)));
      const literal=Buffer.concat([raw.subarray(0,at),Buffer.from('\\ufffd'),raw.subarray(at+14)]);validateCatalog(decodeCatalogResponse(literal));
    }
  }finally{await close(s)}
});
test('Exact names reach the native query unchanged and select their own evidence',async()=>{
  const oracle=JSON.parse(await readFile(path.join(root,'foreign-oracle.json'),'utf8'));
  const s=await startPreview(0,{catalog:path.join(root,'foreign.json'),adapter});
  try{for(const file of oracle.files){const response=await fetch(url(s)+'/catalog-query?'+new URLSearchParams({exact:file.path}));assert.equal(response.status,200);assert.deepEqual((await response.json()).ids,[file.id]);}
    for(const query of ['exact=a&text=b','exact=a&hash=b','exact=a&exact=b','exact=%ff','exact=%ed%a0%80','source=anything']){const response=await fetch(url(s)+'/catalog-query?'+query);assert.equal(response.status,400);await response.text();}
  }finally{await close(s)}
});
test('Scope projection is strict while older catalogs remain unknown',async()=>{
  for(const [name,policy,excluded]of [['off','include-all',0],['on','ignore-exact-ds-store',2],['empty','ignore-exact-ds-store',0],['all-excluded','ignore-exact-ds-store',1],['foreign',null,null]]){
    const s=await startPreview(0,{catalog:path.join(root,name+'.json'),adapter});
    try{const source=await(await fetch(url(s)+'/mode.mjs')).text();const mode=JSON.parse(source.slice(15,-1));assert.equal(mode.ok,true);validateCatalog(mode.catalog);
      if(policy===null)assert.equal(mode.catalog.inventoryScope,null);else{assert.equal(mode.catalog.inventoryScope.policy,policy);assert.equal(mode.catalog.inventoryScope.excludedFiles,excluded);for(const patch of [{version:2},{complete:false},{excludedFiles:-1},{unexpected:1}]){const bad=structuredClone(mode.catalog);Object.assign(bad.inventoryScope,patch);assert.throws(()=>validateCatalog(bad));}}
      const bad=structuredClone(mode.catalog);if(bad.files.length){bad.files[0].path='x'+String.fromCharCode(0xd800);assert.throws(()=>validateCatalog(bad));}
    }finally{await close(s)}
  }
});
