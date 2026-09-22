// Developer-only checks against the extracted package through its launcher.
// Expected counts are fixed by tutorial contents, not computed by the comparer.
import fs from 'node:fs/promises';
import assert from 'node:assert/strict';
export async function packageComparisonChecks({check,evaluate,cdp,screenshot,until,key,catalogs,scoped}) {
  const originals=await Promise.all(catalogs.map(async p=>JSON.parse(await fs.readFile(p,'utf8'))));
  const counts=originals.map(c=>c.files.length);
  const route=async name=>{await evaluate(`location.hash=${JSON.stringify('#'+name)}`);await until(`document.querySelector('h1').textContent===${JSON.stringify(name==='compare'?'Compare recorded snapshots':name==='find'?'Find':'Library')}`);};
  const select=async(selector,index)=>evaluate(`{const s=document.querySelector(${JSON.stringify(selector)});s.selectedIndex=${index};s.dispatchEvent(new Event('change'))}`);
  const settled=()=>until("document.querySelector('#catalog-results-status')?.textContent.includes('recorded file record(s)')");
  const run=async()=>{await evaluate("document.querySelector('#comparison-run').click()");await until("document.querySelector('#comparison-status').textContent.includes('recorded relative keys in the union')");};
  const choose=async index=>select('#comparison-reference',index);
  const click=async name=>evaluate(`[...document.querySelectorAll('#comparison-results .result')].find(r=>r.dataset.key===${JSON.stringify(name)}).click()`);
  await settled();
  await check('Both visible source labels are distinct despite same basename',"document.querySelectorAll('.snapshot-card').length===2&&document.querySelectorAll('.snapshot-card')[0].textContent.includes('Snapshot A')&&document.querySelectorAll('.snapshot-card')[1].textContent.includes('Snapshot B')");
  for(const [index,expected]of [[1,counts[0]],[2,counts[1]],[0,counts[0]+counts[1]]]){
    await select('#snapshot-filter',index);await settled();await check('Packaged snapshot filter '+index,`document.querySelectorAll('.result').length===${expected}`);
  }
  await screenshot('library-all');await route('find');await settled();
  await evaluate("document.querySelector('[aria-label=\"Exact name (JSON string)\"]').click()");await until("document.querySelector('[aria-label=\"Exact recorded name (JSON string)\"]')");
  const special="nested/O'Brien & +%# note.txt";
  await evaluate(`{const f=document.querySelector('[aria-label="Exact recorded name (JSON string)"]');f.value=${JSON.stringify(JSON.stringify(special))};f.dispatchEvent(new Event('input'))}`);await settled();
  await check('Exact punctuation key returns both separately attributed records',"document.querySelectorAll('.result').length===2&&document.querySelectorAll('.result')[0].textContent.includes('Snapshot A')&&document.querySelectorAll('.result')[1].textContent.includes('Snapshot B')");await screenshot('find-both');
  await route('compare');
  await check('No automatic reference or root acceptance',"document.querySelector('#comparison-reference').value===''&&document.querySelector('#comparison-run').disabled&&document.querySelector('#comparison-roots').textContent.includes('No case folding')");
  await choose(1);await run();
  await check('Independent full counts partition twelve recorded keys',`document.querySelectorAll('#comparison-results .result').length===12&&document.querySelector('#comparison-status').textContent.includes('Recorded checksum agreement: ${scoped?7:9}')&&document.querySelector('#comparison-status').textContent.includes('Recorded difference: 1')&&document.querySelector('#comparison-status').textContent.includes('Inconclusive recorded pair: 0')&&document.querySelector('#comparison-status').textContent.includes('Only recorded in reference: ${scoped?3:1}')&&document.querySelector('#comparison-status').textContent.includes('Only recorded in counterpart: 1')`);
  await click(special);
  const expected=originals.map(c=>c.files.find(f=>f.rel_path===special));
  for(const [i,side]of ['reference','counterpart'].entries())await check('Exact '+side+' evidence matches its input',`(()=>{const s=document.querySelector('[data-side=${side}]');return s.dataset.record===${JSON.stringify(String(expected[i].id))}&&s.textContent.includes(${JSON.stringify(expected[i].hash)})&&s.textContent.includes(${JSON.stringify('Exact recorded bytes: '+expected[i].size_bytes)})})()`);
  await evaluate("document.querySelector('#comparison-detail').scrollIntoView()");await screenshot('comparison-detail');
  if(scoped){await click('.DS_Store');await check('Excluded counterpart has scope qualification and no invented record',"document.querySelector('#comparison-detail').textContent.includes('outside the other snapshot')&&document.querySelectorAll('#comparison-detail [data-record]').length===1");await evaluate("document.querySelector('#comparison-detail').scrollIntoView()");await screenshot('scope-qualified');}
  await select('[aria-label="Recorded comparison class"]',2);
  await check('Difference class filter retains full totals but shows one',"document.querySelectorAll('#comparison-results .result').length===1&&document.querySelector('#comparison-status').textContent.includes('Showing 1 keys')");await select('[aria-label="Recorded comparison class"]',0);
  await evaluate("{const f=document.querySelector('[aria-label=\"Filter comparison paths\"]');f.value='café';f.dispatchEvent(new Event('input'))}");
  await check('International path filter is literal',"document.querySelectorAll('#comparison-results .result').length===1&&document.querySelector('#comparison-results').textContent.includes('café notes.txt')");
  await evaluate("{const f=document.querySelector('[aria-label=\"Filter comparison paths\"]');f.value='';f.dispatchEvent(new Event('input'))}");
  await choose(2);await check('Direction change clears old details/counts',"!document.querySelector('#comparison-results .result')&&!document.querySelector('#comparison-detail [data-record]')");await run();await click(special);
  await check('Reversed reference binds the opposite exact evidence',`document.querySelector('[data-side=reference]').dataset.record===${JSON.stringify(String(expected[1].id))}&&document.querySelector('[data-side=reference]').textContent.includes(${JSON.stringify(expected[1].hash)})&&document.querySelector('[data-side=counterpart]').textContent.includes(${JSON.stringify(expected[0].hash)})`);
  await evaluate("document.querySelector('#comparison-detail').scrollIntoView()");await screenshot('comparison-swapped');
  for(const fail of [false,true]){
    await evaluate(`window.originalFetch=fetch;window.held=null;window.fetch=async(...a)=>{const response=await originalFetch(...a);return new Promise((resolve,reject)=>held=()=>${fail?"reject(Error('Injected old error'))":"resolve(response)"})};document.querySelector('#comparison-run').click()`);
    await until("typeof held==='function'");await evaluate('window.fetch=originalFetch');await choose(1);await run();await click(special);
    await evaluate('window.beforeLate=document.querySelector("#comparison-detail").textContent;held()');await evaluate('new Promise(r=>setTimeout(r,60))');
    await check('Late '+(fail?'error':'success')+' cannot change new selection',"document.querySelector('#comparison-detail').textContent===beforeLate");
  }
  await evaluate("document.querySelector('nav a').focus()");await key('Tab',true);await key('Enter');
  await check('Skip retains comparison and focuses main',"document.activeElement.id==='main'&&location.hash==='#compare'");
  await route('library');await settled();await route('compare');await evaluate('history.back()');await until("location.hash==='#library'");await evaluate('history.forward()');await until("location.hash==='#compare'");
  await check('History return requires fresh explicit reference',"document.querySelector('#comparison-reference').value===''&&!document.querySelector('#comparison-results .result')");
  const ax=await cdp('Accessibility.getFullAXTree');assert.ok(ax.nodes.some(n=>n.role?.value==='combobox'&&n.name?.value==='Comparison reference'));
  await choose(1);await run();await evaluate("document.querySelector('#comparison-status').scrollIntoView()");await screenshot('comparison-summary');
}
