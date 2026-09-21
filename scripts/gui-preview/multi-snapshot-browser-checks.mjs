// Real installed-browser controls; the delayed fetch below is explicitly a client
// response-order seam. Normal controls still use the native readers end to end.
import { sourceLabelChecks } from './source-label-browser-checks.mjs';
export async function multiSnapshotChecks({check,evaluate,cdp,screenshot,route,expected}) {
 if(expected.startsWith('multi-labels'))return sourceLabelChecks({check,evaluate,cdp,screenshot,route});
 const until=expr=>evaluate(`new Promise((resolve,reject)=>{let n=0;const t=setInterval(()=>{if(${expr}){clearInterval(t);resolve(true)}else if(++n>300){clearInterval(t);reject(Error('Multi browser timeout: '+${JSON.stringify(expr)}))}},15)})`);
 const settled=()=>until("document.querySelector('#catalog-results-status')?.textContent.includes('recorded file record(s)')");
 const select=async n=>{await evaluate(`{const s=document.querySelector('#snapshot-filter');s.selectedIndex=${n};s.dispatchEvent(new Event('change'));}`);await settled();};
 const input=async(label,value)=>{await evaluate(`{const e=document.querySelector('[aria-label=${JSON.stringify(label)}]');e.value=${JSON.stringify(value)};e.dispatchEvent(new Event('input'));}`);await settled();};
 const key=async(name,shift=false)=>{for(const type of ['keyDown','keyUp'])await cdp('Input.dispatchKeyEvent',{type,key:name,code:name,windowsVirtualKeyCode:name==='Tab'?9:name==='Enter'?13:name==='ArrowDown'?40:38,modifiers:shift?8:0,...(name==='Enter'&&type==='keyDown'?{text:'\r'}:{})});};
 if(expected==='multi-refused'){
  await check('Pair failure never presents one catalog or demo success',"!!document.querySelector('[role=alert]')&&!document.querySelector('.result')&&!document.querySelector('#snapshot-filter')&&!document.body.textContent.includes('Photography NAS')");await screenshot('pair-refused');return;
 }
 await settled();
 await check('Two source cards, fixed selector and no markup execution',"document.querySelectorAll('.snapshot-card').length===2&&document.querySelector('#snapshot-filter').options.length===3&&document.querySelectorAll('#content img,#content script').length===0");
 if(expected==='multi-failure'){
  await evaluate("const q=document.querySelector('[aria-label=\"Recorded SHA-256 prefix\"]');q.value='bbbb';q.dispatchEvent(new Event('input'))");
  await until("document.querySelector('[role=alert]')");
  await check('One failed reader clears All and inspector without fallback',"!document.querySelector('.result')&&!document.querySelector('.inspector')&&document.querySelector('[role=alert]').textContent.includes('previous results cleared')");
  await route('find');await check('Failure remains latched across navigation',"!!document.querySelector('[role=alert]')&&!document.querySelector('.result')");await screenshot('runtime-reader-failed');return;
 }
 await screenshot('pair-library');await route('find');await settled();
 await evaluate("document.querySelector('.snapshot-card summary')?.click()");
 await check('Source provenance is a semantic disclosure',"document.querySelector('.snapshot-card').open===true");
 await evaluate("document.querySelector('.snapshot-card summary')?.click()");
 if(expected==='multi-scopes'){
  await check('Unknown and known zero remain distinct',"document.querySelectorAll('.snapshot-card')[0].textContent.includes('UNKNOWN')&&document.querySelectorAll('.snapshot-card')[0].textContent.includes('Historical inventory time is unknown')&&document.querySelectorAll('.snapshot-card')[1].textContent.includes('0 excluded')");
  await select(1);await screenshot('unknown');await select(2);await screenshot('on-zero');return;
 }
 if(expected==='multi-empty'){
  await check('Genuine empty and all-excluded have separate scope',"document.querySelectorAll('.snapshot-card')[0].textContent.includes('Truly empty source')&&document.querySelectorAll('.snapshot-card')[1].textContent.includes('deliberately excluded')&&document.querySelectorAll('.result').length===0");await screenshot('empty-and-excluded');return;
 }
 if(expected==='multi-controls'){
  await select(1);await evaluate("document.querySelector('[aria-label=\"Exact name (JSON string)\"]').click()");await settled();
  for(const name of ['line\nname','line\rname','line\r\nname',String.raw`line\nname`,' leading ','<img src=x onerror=alert(1)>']){
   await input('Exact recorded name (JSON string)',JSON.stringify(name));await check('Exact control name selects one distinct record '+JSON.stringify(name),"document.querySelectorAll('.result').length===1");await evaluate("document.querySelector('.result').click()");
  }await screenshot('literal-name-inspector');return;
 }
 if(expected==='multi-large'){
  for(const n of [1,2]){await select(n);
   for(const id of ['1','9007199254740991','9007199254740992','9007199254740993','9223372036854775807']){
    await evaluate(`document.querySelector('[data-record="${id}"]').click()`);
    await check('Snapshot '+n+' exact inspector '+id,`document.querySelector('.inspector').textContent.includes('File record ${id} ·')&&document.querySelector('.inspector-snapshot').textContent.includes(document.querySelector('#snapshot-filter').selectedOptions[0].textContent.split(' · ')[0])`);
    if(id==='1')await check('Same ID has input-specific byte evidence '+n,`[...document.querySelectorAll('.inspector dt')].find(d=>d.textContent==='Recorded bytes').nextElementSibling.textContent===${JSON.stringify(n===1?'1':'42')}`);
   }await screenshot('large-inspector-'+n);
  }await select(0);await check('All retains both large identity namespaces',"document.querySelectorAll('.result').length===10");return;
 }
 await check('Mixed scope and recorded time retained separately from load time',"document.querySelector('#content').textContent.includes('exclusion OFF')&&document.querySelector('#content').textContent.includes('exclusion ON')"+(expected==='multi-native'?'':"&&document.querySelector('#content').textContent.includes('2024-02-03T04:05:06-05:00')"));
 await check('All contains five recorded entries, not merged coverage',"document.querySelectorAll('.result').length===5&&document.querySelector('#catalog-results-status').textContent.includes('not unique content')");
 await evaluate('window.multiEvidence=[]');
 for(const n of [1,2]){await select(n);await input('Recorded path contains','shared.txt');await evaluate("document.querySelector('.result').click();window.multiEvidence.push({snapshot:document.querySelector('.inspector-snapshot').textContent,hash:[...document.querySelectorAll('.inspector dt')].find(d=>d.textContent==='Recorded SHA-256').nextElementSibling.textContent})");await screenshot('shared-inspector-'+n);}
 await check('Identical relative name binds each differing hash to its own snapshot',"multiEvidence[0].snapshot!==multiEvidence[1].snapshot&&multiEvidence[0].hash!==multiEvidence[1].hash");
 await input('Recorded path contains','');await select(0);
 // Delay only delivery of a completed real response. The native request is still real.
 await evaluate("window.realFetch=window.fetch;window.held=[];window.fetch=async(...args)=>{const r=await realFetch(...args);if(String(args[0]).includes('text=shared'))return new Promise(resolve=>held.push(()=>resolve(r)));return r;};const q=document.querySelector('[aria-label=\"Recorded path contains\"]');q.value='shared';q.dispatchEvent(new Event('input'));");
 await until('held.length===1');await evaluate('window.fetch=window.realFetch');await select(2);
 await evaluate("document.querySelector('.result').click();window.currentInspector=document.querySelector('.inspector').textContent;held.splice(0).forEach(release=>release())");
 await evaluate('new Promise(r=>setTimeout(r,80))');
 await check('Delayed former All response cannot replace selected snapshot inspector',"document.querySelector('.inspector').textContent===currentInspector&&document.querySelectorAll('.result').length===1");
 await input('Recorded path contains','not-present');await check('Absent record clears inspector without other-snapshot fallback',"document.querySelectorAll('.result').length===0&&!document.querySelector('.inspector').textContent.includes('File record')");
 await input('Recorded path contains','café & +%# note.txt');await check('Native special Unicode name still searchable',"document.querySelectorAll('.result').length===1");
 await evaluate("document.querySelector('.result').click();document.querySelectorAll('.inspector button')[0].click()");await settled();
 await check('Exact name control keeps originating namespace',"document.querySelector('[aria-label=\"Exact name (JSON string)\"]').checked&&document.querySelectorAll('.result').length===1");
 await evaluate("document.querySelector('#snapshot-filter').focus()");await key('ArrowUp');await settled();
 await check('Keyboard selects snapshot filter',"document.querySelector('#snapshot-filter').selectedIndex===1&&document.activeElement.id==='snapshot-filter'");
 await key('Tab');await key('Tab',true);await check('Tab and Shift+Tab return to semantic filter',"document.activeElement.id==='snapshot-filter'");
 await route('library');await settled();await route('find');await settled();
 await evaluate('history.back()');await until("location.hash==='#library'");await settled();
 await evaluate('history.forward()');await until("location.hash==='#find'");await settled();await check('Back/Forward preserve snapshot selection',"document.querySelector('#snapshot-filter').selectedIndex===1");
 await evaluate("document.querySelector('nav a').focus()");await key('Tab',true);await key('Enter');
 await check('Skip link focuses main without workspace reset',"document.activeElement.id==='main'&&document.querySelector('h1').textContent==='Find'&&document.querySelector('#snapshot-filter').selectedIndex===1");
 for(const mode of ['Guided','Studio','Expert']){await evaluate(`document.querySelector('[data-mode="${mode}"]').click()`);await settled();await check(mode+' retains recording warnings',"document.querySelector('#purpose').textContent.includes('verification performed now: unavailable')&&document.querySelector('#scope').textContent.includes('No source/media')");}
 await screenshot('pair-find-final');
 const ax=await cdp('Accessibility.getFullAXTree');if(!ax.nodes.some(n=>n.role?.value==='combobox'&&n.name?.value==='Snapshot view'))throw Error('Accessible snapshot selector missing');
}
