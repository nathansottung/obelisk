// Focused supplement against the already extracted package; no fixture rewrite.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
const inputs=JSON.parse(await fs.readFile(process.env.OBELISK_ALPHA_INPUTS,'utf8'));
const { startPreview }=await import(pathToFileURL(path.join(inputs.packages[0],'preview/server.mjs')));
test('Missing selected catalog remains explicit failure without file creation or static fallback',async()=>{
  const catalog=path.join(inputs.inputs,'intentionally-absent.json');await assert.rejects(fs.stat(catalog),{code:'ENOENT'});
  const server=await startPreview(0,{catalog,adapter:path.join(inputs.packages[0],'bin/obelisk.exe')});const base='http://127.0.0.1:'+server.address().port;
  try{const mode=JSON.parse((await(await fetch(base+'/mode.mjs')).text()).slice(15,-1));assert.equal(mode.enabled,true);assert.equal(mode.ok,false);assert.equal(mode.catalog,undefined);const response=await fetch(base+'/catalog-query?text=');assert.equal(response.status,503);assert.equal((await response.json()).ok,false);}
  finally{await new Promise(resolve=>{server.close(resolve);server.closeAllConnections();});const exit=await server.catalogStopped;assert.equal(exit.code,1);await assert.rejects(fetch(base));}
  await assert.rejects(fs.stat(catalog),{code:'ENOENT'});
});
