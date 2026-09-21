// Test-only generated-source oracle. Never served; producer never reads it.
import { readFile } from 'node:fs/promises';
export async function inventoryChecks({check,evaluate,cdp,screenshot,evidence,writeFile,path}) {
  const oracle=JSON.parse(await readFile(process.env.OBELISK_GUI_INVENTORY_ORACLE,'utf8'));
  const until=expr=>evaluate(`new Promise((resolve,reject)=>{let n=0;const t=setInterval(()=>{if(${expr}){clearInterval(t);resolve(true);}else if(++n>300){clearInterval(t);reject(Error('Inventory browser condition timed out'));}},10);})`);
  const input=(value)=>evaluate(`{const q=document.querySelector('[aria-label="Recorded path contains"]');q.value=${JSON.stringify(value)};q.dispatchEvent(new Event('input'));}`);
  await until(`document.querySelectorAll('.result').length===${oracle.files.length}`);
  await check('Generated source collection and catalog-only notice',`document.querySelector('#content').textContent.includes(${JSON.stringify('Disposable inventory: '+oracle.label)}) && document.querySelector('#scope').textContent.includes('No source/media files are opened')`);
  await check('Inventory does not fabricate storage or backup copies',"document.querySelector('#content').textContent.includes('No storage recorded') && !document.querySelector('.occurrence')");
  await screenshot('inventory-library');
  await evaluate("document.querySelector('nav a[href=\"#find\"]').click()");
  await until(`document.querySelector('h1').textContent==='Find' && document.querySelectorAll('.result').length===${oracle.files.length}`);
  const observed=[];
  for(const file of oracle.files) {
    await input(file.path);
    await until(`document.querySelector('#catalog-results-status').textContent.startsWith(${JSON.stringify(String(oracle.files.filter(f=>f.path.toLowerCase().includes(file.path.toLowerCase())).length)+' recorded')})`);
    const selected=`[...document.querySelectorAll('.result')].find(e=>e.textContent.startsWith(${JSON.stringify(file.path+' · ')}))`;
    await until(selected);
    await evaluate(`${selected}.click()`);
    await check('Observed evidence for '+file.path,`(()=>{const e=document.querySelector('.inspector');const d=Object.fromEntries([...e.querySelectorAll('dt')].map(x=>[x.textContent,x.nextElementSibling.textContent]));return d['Recorded relative path']===${JSON.stringify(file.path)} && d['Source folder (text only)']===${JSON.stringify(oracle.source)} && d['Recorded bytes']===${JSON.stringify(String(file.bytes))} && d['Recorded SHA-256']===${JSON.stringify(file.sha256)} && d['Recorded first-seen time']!=='Unavailable' && e.textContent.includes(${JSON.stringify('File record '+file.id+' ·')}) && document.querySelectorAll('.occurrence').length===0;})()`);
    observed.push(await evaluate("({result:document.querySelector('.result[aria-pressed=true]').textContent,inspector:document.querySelector('.inspector').textContent})"));
    await screenshot('inventory-record-'+file.id);
  }
  // Actual keyboard activation must not reset the selected generated observation.
  const snapshot=()=>evaluate("({hash:location.hash,history:history.length,timeOrigin:performance.timeOrigin,query:document.querySelector('input').value,selected:document.querySelector('.result[aria-pressed=true]').textContent,detail:document.querySelector('.inspector').textContent})");
  await evaluate("document.querySelector('nav a').focus()");
  const key=async(key,shift=false)=>{for(const type of ['keyDown','keyUp'])await cdp('Input.dispatchKeyEvent',{type,key,code:key,windowsVirtualKeyCode:key==='Tab'?9:13,modifiers:shift?8:0,...(key==='Enter'&&type==='keyDown'?{text:'\r'}:{})});};
  await key('Tab',true);await check('Generated snapshot keyboard reaches skip',"document.activeElement.matches('.skip:focus-visible')");
  const before=await snapshot();await key('Enter');
  await check('Generated selection survives skip; main receives focus',`document.activeElement===document.querySelector('main') && ${JSON.stringify(JSON.stringify(before)===JSON.stringify(await snapshot()))}`);
  await input('no-such-generated-observation');await until("document.querySelector('#content').textContent.includes('No recorded matches')");
  await check('No-match cannot retain another generated record',"!document.querySelector('.result,.occurrence') && !document.querySelector('.inspector').textContent.includes('File record')");
  await writeFile(path.join(evidence,'inventory-observed.json'),JSON.stringify({oracle,observed,beforeSkip:before},null,2));
}
