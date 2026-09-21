// Focused R1 regression. Expectations are read from generated input bytes by
// the fixture setup, independently of the UI. Array order is launch order only.
import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';

export async function sourceLabelChecks({check,evaluate,cdp,screenshot,route}) {
  const expectations = JSON.parse(await readFile(process.env.OBELISK_GUI_LABEL_EXPECTATIONS, 'utf8'));
  const until = expression => evaluate(`new Promise((resolve,reject)=>{let n=0;const t=setInterval(()=>{if(${expression}){clearInterval(t);resolve(true)}else if(++n>300){clearInterval(t);reject(Error('Label scenario timeout'))}},15)})`);
  const settled = () => until("document.querySelector('#catalog-results-status')?.textContent.includes('recorded file record(s)')");
  const select = async value => { await evaluate(`{const s=document.querySelector('#snapshot-filter');s.value=${JSON.stringify(value)};s.dispatchEvent(new Event('change'));}`); await settled(); };
  const search = async value => { await evaluate(`{const q=document.querySelector('[aria-label="Recorded path contains"]');q.value=${JSON.stringify(value)};q.dispatchEvent(new Event('input'));}`); await settled(); };
  await settled();
  const bindings = await evaluate("[...document.querySelectorAll('.snapshot-card')].map(c=>({handle:c.dataset.snapshot,digest:c.querySelector('.notice').textContent.match(/[a-f0-9]{64}/)[0]}))");
  assert.equal(bindings.length,2);
  const sources = expectations.map((e,i) => {
    assert.equal(bindings[i].digest,e.digest);
    return {...e,handle:bindings[i].handle,label:`Snapshot ${i===0?'A':'B'} — ${e.basename}`};
  });
  await writeFile(path.join(process.argv[3],'source-associations.json'),JSON.stringify(sources,null,2));
  await evaluate(`window.labelSources=${JSON.stringify(sources)}`);
  await check('Library headings, View controls and scope/time retain distinct startup source labels',`labelSources.every(s=>{const c=document.querySelector('.snapshot-card[data-snapshot="'+s.handle+'"]');return c.querySelector('h2').textContent===s.label&&c.querySelector('button').textContent==='View '+s.label&&c.textContent.includes(s.scope)&&c.textContent.includes(s.time)})`);
  await check('Selector displays both source qualifiers and basenames',"labelSources.every(s=>[...document.querySelector('#snapshot-filter').options].some(o=>o.value===s.handle&&o.textContent===s.label))");
  await screenshot('labels-library');
  await route('find'); await settled();
  await check('Find summaries associate labels with the correct scope and time',"labelSources.every(s=>{const c=document.querySelector('.snapshot-card[data-snapshot=\"'+s.handle+'\"]');return c.querySelector('summary').textContent.startsWith(s.label)&&c.textContent.includes(s.scope)&&c.textContent.includes(s.time)})");
  await check('All rows and per-source counts identify origins before inspector selection',"labelSources.every(s=>[...document.querySelectorAll('.result')].filter(r=>r.dataset.snapshot===s.handle).every(r=>r.textContent.startsWith(s.label+' · '))&&document.querySelector('#catalog-results-status').textContent.includes(s.label+': '+s.count))");
  const ax = await cdp('Accessibility.getFullAXTree');
  for (const s of sources) assert.ok(ax.nodes.some(n=>n.role?.value==='button'&&n.name?.value.startsWith(s.label+' · ')), 'Distinct accessible result source '+s.label);
  await check('Long names wrap without horizontal overflow or a hidden leading qualifier',"document.documentElement.scrollWidth<=innerWidth&&[...document.querySelectorAll('.result')].every(r=>{const range=document.createRange();range.setStart(r.firstChild,0);range.setEnd(r.firstChild,10);const b=range.getBoundingClientRect();return b.left>=0&&b.right<=innerWidth&&getComputedStyle(r).visibility==='visible'})");
  await check('Supported punctuation and catalog text remain inert',"!document.querySelector('#content img,#content script')");
  await screenshot('labels-all');
  const evidence = async (s,context) => {
    await evaluate(`document.querySelector('.result[data-snapshot="${s.handle}"][data-record="${s.record.id}"]').click()`);
    await check(context+' inspector uses fixture evidence and matching visible source',`(()=>{const e=${JSON.stringify(s)},p=document.querySelector('.inspector'),get=k=>[...p.querySelectorAll('dt')].find(d=>d.textContent===k)?.nextElementSibling.textContent;return p.querySelector('.inspector-snapshot').textContent.includes(e.label)&&p.querySelector('.inspector-snapshot').textContent.includes(e.digest)&&get('Recorded bytes')===e.record.bytes&&get('Recorded SHA-256')===e.record.hash&&get('Recorded relative path')===e.record.path&&get('Recorded first-seen time')===e.record.firstSeen})()`);
  };
  for (const s of sources) { await evidence(s,'All'); await screenshot('labels-inspector-'+sources.indexOf(s)); }
  for (const s of sources) { await select(s.handle); await evidence(s,'Own filter'); }
  await select('all'); await search('not-present-label-regression');
  await check('No-result counts still identify both sources',"!document.querySelector('.result')&&labelSources.every(s=>document.querySelector('#catalog-results-status').textContent.includes(s.label+': 0'))");
  await screenshot('labels-no-results'); await search('');
  await evaluate("window.originalLabelFetch=window.fetch;window.labelHeld=[];window.fetch=async(...a)=>{const r=await originalLabelFetch(...a);return new Promise(resolve=>labelHeld.push(()=>resolve(r)))};document.querySelector('[aria-label=\"Recorded path contains\"]').dispatchEvent(new Event('input'))");
  await until('labelHeld.length===1'); await evaluate('window.fetch=originalLabelFetch');
  await select(sources[1].handle); await evidence(sources[1],'New selection');
  await evaluate('window.labelInspector=document.querySelector(".inspector").textContent;labelHeld.splice(0).forEach(f=>f())');
  await evaluate('new Promise(r=>setTimeout(r,80))');
  await check('Delayed former All result cannot relabel newer selection',"document.querySelector('.inspector').textContent===labelInspector");
  await route('library'); await settled(); await route('find'); await settled();
  await evaluate('history.back()'); await until("location.hash==='#library'"); await settled();
  await evaluate('history.forward()'); await until("location.hash==='#find'"); await settled();
  await evidence(sources[1],'History');
  const key = async (name,shift=false) => {for(const type of ['keyDown','keyUp'])await cdp('Input.dispatchKeyEvent',{type,key:name,code:name,windowsVirtualKeyCode:name==='Tab'?9:13,modifiers:shift?8:0,...(name==='Enter'&&type==='keyDown'?{text:'\r'}:{})});};
  await evaluate("document.querySelector('#snapshot-filter').focus()"); await key('Tab'); await key('Tab',true);
  await check('Tab and Shift+Tab retain semantic filter focus',"document.activeElement.id==='snapshot-filter'");
  await evaluate("document.querySelector('nav a').focus()"); await key('Tab',true); await key('Enter');
  await check('Skip link retains Find and source-qualified selection',`document.activeElement.id==='main'&&document.querySelector('h1').textContent==='Find'&&document.querySelector('#snapshot-filter').value===${JSON.stringify(sources[1].handle)}`);
}
