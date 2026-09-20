// Author regressions driven by the installed-Chrome harness; never served.
export async function correctionChecks({check,evaluate,screenshot,expected,evidence,writeFile,path}) {
  const until = expression => evaluate(`new Promise((resolve,reject)=>{let n=0;const timer=setInterval(()=>{if(${expression}){clearInterval(timer);resolve(true);}else if(++n>200){clearInterval(timer);reject(Error('Browser condition timed out'));}},10);})`);
  const input = (label,value) => evaluate(`{const e=document.querySelector('[aria-label=${JSON.stringify(label)}]');e.value=${JSON.stringify(value)};e.dispatchEvent(new Event('input'));}`);
  const observed=[];
  if(expected.startsWith('exact') || expected==='review-integers') {
    await until("document.querySelector('.result')");
    await evaluate("document.querySelector('nav a[href=\"#find\"]').click()");
    await until("document.querySelector('h1').textContent==='Find' && document.querySelector('.result')");
    const ids=expected==='review-integers'?['9007199254740992','9007199254740993']:['1','9007199254740991','9007199254740992','9007199254740993','9223372036854775807'];
    for(const [i,id] of ids.entries()) {
      const hash=String.fromCharCode(97+i).repeat(64);
      const folder=expected==='review-integers'?'/synthetic-never-open/ALPHA/'+(i?'two':'one'):'/synthetic-never-open/EXACT/'+id;
      const bytes=expected==='review-integers'&&!i?'123':id;
      await input('Recorded SHA-256 prefix',hash);
      await until(`document.querySelectorAll('.result').length===1 && document.querySelector('.result').textContent.includes(${JSON.stringify('Record '+id)})`);
      await evaluate("document.querySelector('.result').click()");
      await check('Exact record '+id+' selects its own hash, source and size',`(()=>{const e=document.querySelector('.inspector');const values=Object.fromEntries([...e.querySelectorAll('dt')].map(dt=>[dt.textContent,dt.nextElementSibling.textContent]));return e.textContent.includes(${JSON.stringify('File record '+id+' ·')}) && values['Recorded SHA-256']===${JSON.stringify(hash)} && values['Source folder (text only)']===${JSON.stringify(folder)} && values['Recorded bytes']===${JSON.stringify(bytes)};})()`);
      if(expected!=='review-integers')await check('Exact copy join '+id,`document.querySelector('.occurrence summary').textContent.includes(${JSON.stringify('chunk-'+id+'-copy-0')})`);
      observed.push(await evaluate("({query:document.querySelector('[aria-label=\"Recorded SHA-256 prefix\"]').value,result:document.querySelector('.result').textContent,inspector:document.querySelector('.inspector').textContent})"));
      if(i>=ids.length-2)await screenshot('exact-inspector-'+id);
    }
    await input('Recorded path contains','no-match');await until("document.querySelector('#content').textContent.includes('No recorded matches')");
    await check('No-match never defaults to the first exact record',"document.querySelectorAll('.result,.occurrence').length===0 && !document.querySelector('.inspector').textContent.includes('File record')");
  } else {
    if(['failure-query','failure-invalid-id','failure-late'].includes(expected)) {
      await until("document.querySelectorAll('.result').length===2");
      await evaluate("document.querySelector('.result').click();document.querySelector('.occurrence').open=true");
      await check('Genuine prior query and selected evidence exist before fault',"document.querySelector('.inspector').textContent.includes('aaaaaaaaaaaaaaaa') && document.querySelector('.occurrence').open");
      observed.push(await evaluate("({stage:'before',content:document.querySelector('#content').textContent})"));
      if(expected==='failure-late') {
        await evaluate("{const fetchBefore=window.fetch;window.__correctionRequests=[];window.fetch=(...args)=>{const record={url:String(args[0]),done:false};window.__correctionRequests.push(record);return fetchBefore(...args).then(response=>{response.clone().text().then(()=>{record.done=true;});return response;});};}");
        await input('Recorded path contains','delay');await until("window.__correctionRequests.some(r=>r.url.includes('delay'))");
      }
      await input('Recorded SHA-256 prefix','bbbb');
    }
    await until("document.querySelector('[role=alert]')");
    await check('Failed read clears identity, collections, results and selected evidence',"!document.querySelector('#catalog-identity,.result,.inspector,.occurrence') && !document.querySelector('#content').textContent.includes('ALPHA') && !document.body.textContent.includes('Photography NAS') && document.querySelector('[role=alert]').textContent.includes('Relaunch')");
    if(expected==='failure-late') {
      await until("window.__correctionRequests.every(r=>r.done)");
      await check('Late superseded response cannot resurrect failed view',"!!document.querySelector('[role=alert]') && !document.querySelector('.result,#catalog-identity')");
    }
    await screenshot(expected);
    await evaluate("document.querySelector('[data-mode=Expert]').click()");
    await check('Disclosure rerender cannot restore failed bootstrap',"!!document.querySelector('[role=alert]') && !document.querySelector('.result,#catalog-identity')");
    observed.push(await evaluate("({stage:'failed',content:document.querySelector('#content').textContent})"));
  }
  await writeFile(path.join(evidence,'correction-observed.json'),JSON.stringify(observed,null,2));
}
