import {readFile,writeFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import path from 'node:path';
import assert from 'node:assert/strict';
export async function comparisonChecks({check,evaluate,cdp,screenshot,route,expected}) {
 const until=e=>evaluate(`new Promise((resolve,reject)=>{let n=0;const t=setInterval(()=>{if(${e}){clearInterval(t);resolve(true)}else if(++n>400){clearInterval(t);reject(Error('Comparison browser timeout'))}},20)})`);
 const done=()=>until("document.querySelector('#comparison-status')?.textContent.includes('recorded relative keys in the union')");
 const select=async n=>evaluate(`{const s=document.querySelector('#comparison-reference');s.selectedIndex=${n};s.dispatchEvent(new Event('change'));}`);
 const run=async()=>{await evaluate("document.querySelector('#comparison-run').click()");await done();};
 const filter=async value=>evaluate(`{const e=document.querySelector('[aria-label="Filter comparison paths"]');e.value=${JSON.stringify(value)};e.dispatchEvent(new Event('input'));}`);
 const mode=await evaluate("import('/mode.mjs').then(m=>m.default)");assert.equal(mode.ok,true);
 // Hash raw inputs, never parse their potentially large native numeric IDs in JS.
 if(!expected.includes('failure'))for(const [i,file]of [process.argv[4],process.argv[7]].entries())assert.equal(mode.snapshots[i].catalog.digest,createHash('sha256').update(await readFile(file)).digest('hex'));
 await writeFile(path.join(process.argv[3],'comparison-startup.json'),JSON.stringify(mode,null,2));
 await route('compare');
 await check('Explicit reference and alignment required; no default authoritative input',"document.querySelector('#comparison-reference').value===''&&document.querySelector('#comparison-run').disabled&&document.querySelector('#comparison-roots').textContent.includes('No case folding')");
 await evaluate("document.querySelector('[aria-label=\"Recorded comparison\"]').scrollIntoView()");await screenshot('comparison-before');
 if(expected==='compare-refused') {await check('Unsupported frame refused while return links remain usable',"!!document.querySelector('[role=alert]')&&document.querySelector('#comparison-run').disabled&&!!document.querySelector('a[href=\"#find\"]')");await screenshot('comparison-refused');return;}
 await select(1);
 if(expected==='compare-failure') {await evaluate("document.querySelector('#comparison-run').click()");await until("document.querySelector('#comparison-status').textContent.includes('unavailable')");await check('Reader failure has no successful totals, rows or stale evidence',"!document.querySelector('#comparison-results .result')&&!document.querySelector('#comparison-detail [data-record]')");await screenshot('comparison-reader-failed');return;}
 await run();
 if(expected==='compare-scope'||expected==='compare-empty'){
  await check('Scope/time remain per source and no health percentage appears',"document.querySelectorAll('.snapshot-card').length===2&&!document.querySelector('#comparison-status').textContent.includes('100%')");
  if(expected==='compare-empty')await check('Empty/all-excluded counts are zero keys, not verification',"document.querySelector('#comparison-status').textContent.startsWith('0 recorded relative keys')");
  else {await evaluate("[...document.querySelectorAll('#comparison-results .result')].find(r=>r.dataset.key==='.DS_Store').click()");await check('One-sided .DS_Store retains exclusion qualification and no invented opposite ID',"document.querySelector('#comparison-detail').textContent.includes('outside')&&document.querySelectorAll('#comparison-detail [data-record]').length===1");}
  await screenshot(expected);return;
 }
 const reversed=expected==='compare-reverse';
 await evaluate("{const s=document.querySelector('[aria-label=\"Recorded comparison class\"]');s.value='difference';s.dispatchEvent(new Event('change'));}");
 await check('Recorded class filter selects the three independent differences',"document.querySelectorAll('#comparison-results .result').length===3");
 await evaluate("{const s=document.querySelector('[aria-label=\"Recorded comparison class\"]');s.value='';s.dispatchEvent(new Event('change'));}");
 await check('Independent five-class table partitions twenty exact keys',"document.querySelectorAll('#comparison-results .result').length===20&&document.querySelector('#comparison-status').textContent.includes('Recorded checksum agreement: 7')&&document.querySelector('#comparison-status').textContent.includes('Recorded difference: 3')&&document.querySelector('#comparison-status').textContent.includes('Inconclusive recorded pair: 2')");
 await check('Same-basename roots remain visibly distinguishable',"document.querySelector('#comparison-roots').textContent.includes('Snapshot A —')&&document.querySelector('#comparison-roots').textContent.includes('Snapshot B —')&&!document.querySelector('#content img,#content script')");
 const click=async key=>evaluate(`[...document.querySelectorAll('#comparison-results .result')].find(r=>r.dataset.key===${JSON.stringify(key)}).click()`);
 await click('large');
 await check('Both exact large IDs and sizes match independent native expectations',`(()=>{const a=document.querySelector('[data-side=reference]'),b=document.querySelector('[data-side=counterpart]');return a.dataset.record==='${reversed?'9007199254740994':'9007199254740993'}'&&b.dataset.record==='${reversed?'9007199254740993':'9007199254740994'}'&&a.textContent.includes('Exact recorded bytes: ${reversed?'9007199254740993':'9007199254740992'}')&&b.textContent.includes('Exact recorded bytes: ${reversed?'9007199254740992':'9007199254740993'}')})()`);
 await evaluate("document.querySelector('#comparison-detail').scrollIntoView()");await screenshot('comparison-large-detail');
 await click('contradiction');await check('Matching digest with unequal sizes is inconclusive',"document.querySelector('#comparison-detail').textContent.includes('Inconsistent recorded evidence')");
 await click('different');await check('Both full recorded checksums retain their own side',"document.querySelector('#comparison-detail').textContent.includes('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')&&document.querySelector('#comparison-detail').textContent.includes('bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb')");
 await click('only-reference');await check('One-sided row has no invented other record',`document.querySelectorAll('#comparison-detail [data-record]').length===1&&document.querySelector('[data-side=${reversed?'reference':'counterpart'}]').textContent.includes('No recorded row')`);
 await evaluate("document.querySelector('[aria-label=\"Exact comparison path\"]').click()");
 for(const name of ['line\nname','line\rname','line\r\nname','line\\nname']){await filter(JSON.stringify(name));await check('Exact control path remains distinct '+JSON.stringify(name),"document.querySelectorAll('#comparison-results .result').length===1");await click(name);}
 await evaluate("document.querySelector('[aria-label=\"Exact comparison path\"]').click()");await filter('not-present');await check('No-results clears prior selection',"!document.querySelector('#comparison-results .result')&&!document.querySelector('#comparison-detail [data-record]')");await filter('');
 await select(2);await check('Reference swap immediately invalidates results and selection',"!document.querySelector('#comparison-results .result')&&!document.querySelector('#comparison-detail [data-record]')");await run();
 await check('Direction reverses only-reference membership',`[...document.querySelectorAll('#comparison-results .result')].find(r=>r.dataset.key==='only-reference').dataset.kind==='${reversed?'referenceOnly':'counterpartOnly'}'`);
 for(const failure of [false,true]){
  await evaluate(`window.compareFetch=window.fetch;window.heldComparison=[];window.fetch=async(...a)=>{const r=await compareFetch(...a);return new Promise((resolve,reject)=>heldComparison.push(()=>${failure?"reject(Error('Injected late error'))":"resolve(r)"}))};document.querySelector('#comparison-run').click()`);
  await until('heldComparison.length===1');await evaluate('window.fetch=compareFetch');await select(1);await run();await click('agreement');
  await evaluate('window.newEvidence=document.querySelector("#comparison-detail").textContent;heldComparison.splice(0).forEach(f=>f())');await evaluate('new Promise(r=>setTimeout(r,80))');
  await check('Late '+(failure?'error':'success')+' cannot overwrite current reference/evidence',"document.querySelector('#comparison-detail').textContent===newEvidence");
 }
 await route('library');await route('compare');await evaluate('history.back()');await until("location.hash==='#library'");await evaluate('history.forward()');await until("location.hash==='#compare'");
 await check('History restoration requires fresh explicit direction/action',"document.querySelector('#comparison-reference').value===''&&!document.querySelector('#comparison-results .result')");
 const key=async(name,shift=false)=>{for(const type of ['keyDown','keyUp'])await cdp('Input.dispatchKeyEvent',{type,key:name,code:name,windowsVirtualKeyCode:name==='Tab'?9:13,modifiers:shift?8:0,...(name==='Enter'&&type==='keyDown'?{text:'\r'}:{})});};
 await evaluate("document.querySelector('#comparison-reference').focus()");await key('Tab');await key('Tab',true);await check('Keyboard returns to named reference selector',"document.activeElement.id==='comparison-reference'");
 await evaluate("document.querySelector('nav a').focus()");await key('Tab',true);await key('Enter');await check('Skip link retains Compare and focuses main',"document.activeElement.id==='main'&&document.querySelector('h1').textContent==='Compare recorded snapshots'");
 await select(1);await run();await click('agreement');await evaluate("document.querySelector('#comparison-status').scrollIntoView()");await screenshot('comparison-results');
 const ax=await cdp('Accessibility.getFullAXTree');assert.ok(ax.nodes.some(n=>n.role?.value==='combobox'&&n.name?.value==='Comparison reference'));await writeFile(path.join(process.argv[3],'comparison-accessibility.json'),JSON.stringify(ax,null,2));
 // Explicit protocol seam: a response with another native ID must not select data.
 await evaluate("window.realComparisonFetch=fetch;window.fetch=async(...a)=>{const r=await realComparisonFetch(...a),v=await r.json();v.rows[0].referenceId='999';return new Response(JSON.stringify(v),{status:200})};document.querySelector('#comparison-run').click()");
 await until("document.querySelector('#comparison-status').textContent.includes('unavailable')");await check('Mismatched response ID clears selection and refuses complete result',"!document.querySelector('#comparison-results .result')&&!document.querySelector('#comparison-detail [data-record]')");await screenshot('comparison-invalid-response');
}
