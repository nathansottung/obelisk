import { readFile } from 'node:fs/promises';
import { displayName } from './catalog-names.mjs';
export async function inventoryCorrectionChecks({check,evaluate,cdp,screenshot,evidence,writeFile,path}) {
  const oracle=JSON.parse(await readFile(process.env.OBELISK_GUI_INVENTORY_ORACLE,'utf8'));
  const wait=condition=>evaluate(`new Promise((resolve,reject)=>{let n=0;const t=setInterval(()=>{if(${condition}){clearInterval(t);resolve(true)}else if(++n>400){clearInterval(t);reject(Error('Correction browser condition timed out'))}},10)})`);
  await wait(`document.querySelectorAll('.result').length===${oracle.files.length} && document.querySelector('#catalog-results-status')?.textContent.includes('recorded file')`);
  const scope=oracle.scope;
  await check('Durable scope after fresh catalog load',scope
    ? `document.querySelector('#inventory-scope').textContent.includes(${JSON.stringify('.DS_Store exclusion '+(scope.policy==='include-all'?'OFF':'ON'))}) && document.querySelector('#inventory-scope').textContent.includes(${JSON.stringify(scope.excludedFiles+' excluded')}) && document.querySelector('#inventory-scope').textContent.includes(${JSON.stringify(scope.entries+' entries visited')})`
    : `document.querySelector('#inventory-scope').textContent.includes('UNKNOWN')`);
  if(scope?.entries===0)await check('Truly empty source disclosure',`document.querySelector('#inventory-scope').textContent.includes('Truly empty source')`);
  if(scope?.excludedFiles>0&&scope.includedFiles===0)await check('All-excluded distinct from empty',`document.querySelector('#inventory-scope').textContent.includes('All observed regular files deliberately excluded')`);
  await screenshot('scope-library');
  if(!oracle.files.length)return;
  await evaluate(`document.querySelector('nav a[href="#find"]').click()`);
  await wait(`location.hash==='#find' && document.querySelectorAll('.result').length===${oracle.files.length}`);
  // Observer only: clone actual server responses; never substitute queries/results.
  await evaluate(`{window.observedNameQueries=[];const original=window.fetch;window.fetch=async(...args)=>{const response=await original(...args);if(String(args[0]).startsWith('/catalog-query'))window.observedNameQueries.push({url:String(args[0]),response:await response.clone().json()});return response;};}`);
  await evaluate(`document.querySelector('[aria-label="Exact name (JSON string)"]').click()`);
  await wait(`document.querySelector('[aria-label="Exact recorded name (JSON string)"]')`);
  const enter=async text=>{
    await evaluate(`document.querySelector('[aria-label="Exact recorded name (JSON string)"]').focus()`);
    await cdp('Input.dispatchKeyEvent',{type:'keyDown',key:'a',code:'KeyA',windowsVirtualKeyCode:65,modifiers:2});
    await cdp('Input.dispatchKeyEvent',{type:'keyUp',key:'a',code:'KeyA',windowsVirtualKeyCode:65,modifiers:2});
    await cdp('Input.insertText',{text}); // Browser insertion simulation, not native clipboard qualification.
    await cdp('Input.dispatchKeyEvent',{type:'keyDown',key:'Enter',code:'Enter',windowsVirtualKeyCode:13,text:'\r'});
    await cdp('Input.dispatchKeyEvent',{type:'keyUp',key:'Enter',code:'Enter',windowsVirtualKeyCode:13});
  };
  for(const f of oracle.files){
    await enter(JSON.stringify(f.path));
    await wait(`document.querySelectorAll('.result').length===1 && document.querySelector('.result')?.textContent.endsWith(${JSON.stringify('Record '+f.id)})`);
    await evaluate(`document.querySelector('.result').click()`);
    await check('Exact semantic query and native selection '+JSON.stringify(f.path),`(()=>{const q=window.observedNameQueries.at(-1);const params=new URL(q.url,location.origin).searchParams;const d=Object.fromEntries([...document.querySelectorAll('.inspector dt')].map(e=>[e.textContent,e.nextElementSibling.textContent]));return params.get('exact')===${JSON.stringify(f.path)} && q.response.ok && JSON.stringify(q.response.ids)===${JSON.stringify(JSON.stringify([f.id]))} && d['Recorded relative path']===${JSON.stringify(displayName(f.path))} && d['Recorded bytes']===${JSON.stringify(String(f.bytes))} && d['Recorded SHA-256']===${JSON.stringify(f.sha256)};})()`);
    if(oracle.files.length<16 || /[\r\n]/.test(f.path))await screenshot('exact-record-'+f.id);
  }
  const snapshot=()=>evaluate(`({entry:document.querySelector('[aria-label="Exact recorded name (JSON string)"]').value,selected:document.querySelector('.result[aria-pressed=true]').textContent,detail:document.querySelector('.inspector').textContent,time:performance.timeOrigin})`);
  const before=await snapshot();
  for(const mode of ['Expert','Guided','Studio']){await evaluate(`[...document.querySelectorAll('[data-mode]')].find(e=>e.textContent===${JSON.stringify(mode)}).click()`);await wait(`document.querySelector('.result[aria-pressed=true]')`);await check('Exact state survives '+mode,JSON.stringify(JSON.stringify(before)===JSON.stringify(await snapshot())));}
  await evaluate('history.back()');await wait(`document.querySelector('h1').textContent==='Library' && document.querySelector('.result[aria-pressed=true]')`);
  await evaluate('history.forward()');await wait(`document.querySelector('h1').textContent==='Find' && document.querySelector('.result[aria-pressed=true]')`);
  await check('Exact query and evidence survive Back/Forward',JSON.stringify(JSON.stringify(before)===JSON.stringify(await snapshot())));
  await evaluate(`document.querySelector('nav a').focus()`);
  for(const type of ['keyDown','keyUp'])await cdp('Input.dispatchKeyEvent',{type,key:'Tab',code:'Tab',windowsVirtualKeyCode:9,modifiers:8});
  await check('Keyboard reaches skip link',`document.activeElement.matches('.skip:focus-visible')`);
  for(const type of ['keyDown','keyUp'])await cdp('Input.dispatchKeyEvent',{type,key:'Enter',code:'Enter',windowsVirtualKeyCode:13,...(type==='keyDown'?{text:'\r'}:{})});
  await check('Skip focuses main without resetting exact state',`document.activeElement===document.querySelector('main') && ${JSON.stringify(JSON.stringify(before)===JSON.stringify(await snapshot()))}`);
  const oldCount=await evaluate('window.observedNameQueries.length');
  await enter('"\\ud800"');await wait(`document.querySelector('#name-entry-error').textContent.length>0`);
  await evaluate('new Promise(r=>setTimeout(r,180))');
  await check('Invalid encoded entry clears results and sends no repaired query',`window.observedNameQueries.length===${oldCount} && !document.querySelector('.result') && !document.querySelector('.inspector').textContent.includes('File record')`);
  await enter(JSON.stringify(oracle.files[0].path));await wait(`document.querySelectorAll('.result').length===1`);
  await check('Valid input recovers without clearing failed-load protection',`document.querySelector('#name-entry-error').textContent==='' && !document.querySelector('#content img,#content script')`);
  const literal=oracle.files.find(f=>f.path===String.raw`line\nname.txt`);
  if(literal){
    await evaluate(`document.querySelector('[aria-label="Exact name (JSON string)"]').click();document.querySelector('[aria-label="Recorded path contains"]').focus()`);
    await cdp('Input.insertText',{text:literal.path});
    await cdp('Input.dispatchKeyEvent',{type:'keyDown',key:'Enter',code:'Enter',windowsVirtualKeyCode:13,text:'\r'});
    await cdp('Input.dispatchKeyEvent',{type:'keyUp',key:'Enter',code:'Enter',windowsVirtualKeyCode:13});
    await wait(`document.querySelectorAll('.result').length===1 && document.querySelector('.result').textContent.endsWith(${JSON.stringify('Record '+literal.id)})`);
    await check('Ordinary input keeps literal backslash-n distinct from LF',`(()=>{const q=window.observedNameQueries.at(-1);const p=new URL(q.url,location.origin).searchParams;return !p.has('exact') && p.get('text')===${JSON.stringify(literal.path)} && JSON.stringify(q.response.ids)===${JSON.stringify(JSON.stringify([literal.id]))};})()`);
    await screenshot('ordinary-literal-backslash');
  }
  await writeFile(path.join(evidence,'exact-query-observations.json'),JSON.stringify({oracle,before,queries:await evaluate('window.observedNameQueries'),entryMethod:'CDP keyboard select-all, Input.insertText, Enter; simulated insertion, not OS clipboard'},null,2));
}
