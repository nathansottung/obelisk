// Runs inside the existing installed-Chrome/CDP harness; never served to browsers.
import { correctionChecks } from './catalog-correction-browser-checks.mjs';
export async function catalogChecks({ check, evaluate, cdp, screenshot, route, expected, evidence, writeFile, path }) {
  if(expected.startsWith('exact') || expected.startsWith('failure-') || expected==='review-integers') {
    return correctionChecks({check,evaluate,screenshot,expected,evidence,writeFile,path});
  }
  const settle = () => evaluate('new Promise(r=>setTimeout(r,250))');
  if (expected === 'refused') {
    await check('Refused catalog has explicit failure, no demo fallback', "document.querySelector('[role=alert]').textContent.includes('Catalog load refused') && !document.querySelector('.result') && !document.body.textContent.includes('Photography NAS')");
    await screenshot('catalog-refused'); return;
  }
  await check('Catalog notice and digest distinguish load from scan', "document.querySelector('#scope').textContent.includes('No source/media files are opened') && document.querySelector('#catalog-identity').textContent.includes('Session load time') && !document.body.textContent.includes('no files are read or written')");
  await settle();
  if (expected === 'empty') {
    await check('Valid empty representation is not damaged input', "document.querySelector('#content').textContent.includes('Valid empty catalog') && !document.querySelector('[role=alert]')");
    await screenshot('catalog-empty'); return;
  }
  await check('Input-specific persisted names and result count', `document.querySelectorAll('.result').length===2 && [...document.querySelectorAll('.result')].every(e=>e.textContent.includes(${JSON.stringify(expected)})) && !document.body.textContent.includes('Photography NAS')`);
  await check('Persisted markup remains inert text', "document.querySelector('#content').textContent.includes('<img src=x onerror=alert(1)>') && document.querySelectorAll('img').length===0");
  await evaluate("document.querySelector('.result').click();document.querySelector('.occurrence').open=true");
  await check('Distinct native records and two current-content occurrences', "document.querySelectorAll('.occurrence').length===2 && document.querySelector('.inspector').textContent.includes('aaaaaaaaaaaaaaaa') && document.querySelector('.inspector').textContent.includes('Recorded verification result: Unavailable')");
  await screenshot('catalog-library-inspector');
  const key = async (name,shift=false) => {for(const type of ['keyDown','keyUp'])await cdp('Input.dispatchKeyEvent',{type,key:name,code:name,windowsVirtualKeyCode:name==='Tab'?9:13,modifiers:shift?8:0,...(name==='Enter'&&type==='keyDown'?{text:'\r'}:{})});await settle();};
  const probes=[];
  for(const slug of ['library','find','find-rerender']) {
    if(slug==='find') {await evaluate("document.querySelector('nav a[href=\"#find\"]').click()");await settle();}
    if(slug==='find-rerender'){await evaluate("document.querySelector('[data-mode=Expert]').click()");await settle();}
    await evaluate(`{const q=document.querySelector('[aria-label="Recorded path contains"]');q.value=${JSON.stringify(expected)};q.dispatchEvent(new Event('input'));}`);await settle();
    await evaluate("document.querySelector('.result').click();document.querySelector('.occurrence').open=true;document.querySelector('nav a').focus()");await key('Tab',true);
    const snapshot=()=>evaluate("({url:location.href,history:history.length,timeOrigin:performance.timeOrigin,query:document.querySelector('input').value,selection:document.querySelector('.result[aria-pressed=true]').textContent,detail:document.querySelector('.inspector').textContent,open:document.querySelector('.occurrence').open})");
    await check(slug+' keyboard reaches visible skip',"document.activeElement.matches('.skip:focus-visible') && document.activeElement.getBoundingClientRect().top>=0");
    const before=await snapshot();await screenshot('catalog-skip-link-'+slug);await key('Enter');const after=await snapshot();
    await check(slug+' real main focus and current state', `document.activeElement===document.querySelector('main') && ${JSON.stringify(JSON.stringify(before)===JSON.stringify(after))}`);
    await screenshot('catalog-skip-main-'+slug);await key('Tab');
    await check(slug+' Tab enters workspace',"document.activeElement===document.querySelector('[data-mode=Guided]')");await key('Tab',true);
    await check(slug+' ShiftTab is usable',"document.activeElement.textContent==='Settings'");
    probes.push({slug,before,after});
  }
  await writeFile(path.join(evidence,'catalog-keyboard.json'),JSON.stringify(probes,null,2));
  await evaluate("{const q=document.querySelector('[aria-label=\"Recorded path contains\"]');q.value='missing';q.dispatchEvent(new Event('input'));}");await settle();
  await check('No results clears selected evidence',"document.querySelectorAll('.result,.occurrence').length===0 && document.querySelector('#content').textContent.includes('No recorded matches')");
  await evaluate("{const q=document.querySelector('[aria-label=\"Recorded path contains\"]');q.value='';q.dispatchEvent(new Event('input'));const h=document.querySelector('[aria-label=\"Recorded SHA-256 prefix\"]');h.value='bbbb';h.dispatchEvent(new Event('input'));}");await settle();
  await check('Native hash prefix query selects second file record',"document.querySelectorAll('.result').length===1 && document.querySelector('.result').textContent.includes('Record 2')");
  await evaluate("document.querySelector('.result').click()");
  await check('Second duplicate filename has distinct source and content evidence',`document.querySelector('.inspector').textContent.includes(${JSON.stringify('/synthetic-never-open/'+expected+'/two')}) && document.querySelector('.inspector').textContent.includes('bbbbbbbbbbbbbbbb') && document.querySelectorAll('.occurrence').length===0`);
  await screenshot('catalog-find-second');
  await evaluate("document.querySelector('nav a[href=\"#library\"]').click()");await settle();await evaluate('history.back()');await settle();
  await check('Back retains Find route',"location.hash==='#find' && document.querySelector('h1').textContent==='Find'");await evaluate('history.forward()');await settle();
  await check('Forward returns Library',"location.hash==='#library' && document.querySelector('h1').textContent==='Library'");
}
