// Developer-only browser rehearsal of the extracted launcher and runtime.
import fs from 'node:fs/promises';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import assert from 'node:assert/strict';
import { packageComparisonChecks } from './comparison-browser-checks.mjs';
const [inputsFile, chrome, evidence, selectedCase] = process.argv.slice(2);
if(!inputsFile||!chrome||!evidence||!path.isAbsolute(evidence))throw Error('Provide rehearsal-inputs.json, installed Chrome and NEW absolute evidence directory');
const inputs=JSON.parse(await fs.readFile(inputsFile,'utf8'));await fs.mkdir(evidence);
const env=Object.fromEntries(['SystemRoot','WINDIR','LOCALAPPDATA','APPDATA','USERPROFILE','TEMP','TMP','COMSPEC','PATHEXT'].map(k=>[k,process.env[k]]));env.PATH=path.dirname(process.execPath)+path.delimiter+path.join(process.env.SystemRoot,'System32');
const cases=[['off',inputs.packages[0],path.join(inputs.workspaces[0],'catalogs/off.json')],['on',inputs.packages[1],path.join(inputs.workspaces[1],'catalogs/on.json')],['foreign',inputs.packages[1],path.join(inputs.inputs,'foreign.json')],['refused',inputs.packages[0],path.join(inputs.inputs,'scope-refused.json')],['static',inputs.packages[1],null]];
if(inputs.pairs)cases.push(['pair',inputs.packages[0],inputs.pairs[0].a,inputs.pairs[0].b],['pair-reversed',inputs.packages[1],inputs.pairs[1].b,inputs.pairs[1].a],['pair-scope',inputs.packages[1],inputs.pairs[1].a,inputs.pairs[1].on],['pair-reopen',inputs.packages[0],inputs.pairs[0].a,inputs.pairs[0].b]);
if(selectedCase&&!cases.some(([name])=>name===selectedCase))throw Error('Unknown selected browser case');
const summary=[];const delay=ms=>new Promise(r=>setTimeout(r,ms));
for(const [name,pkg,catalog,secondCatalog]of cases.filter(([name])=>!selectedCase||name===selectedCase)){
  const dir=path.join(evidence,name);await fs.mkdir(dir);const started=Date.now(),checks=[],requests=[],errors=[],pending=new Map();let server,browser,socket,browserExit,serverExit,serverText='',serverError='',browserText='',seq=0,base,failed,forcedBrowser=false,forcedServer=false;
  const guard=setTimeout(()=>{forcedBrowser=true;browser?.kill();server?.stdin.write('stop\n');},60000);
  try{
    server=spawn(process.execPath,[path.join(pkg,'launcher.mjs'),...(catalog?['view',catalog,...(secondCatalog?[secondCatalog]:[])]:['static'])],{cwd:inputs.cwd,env,windowsHide:true,stdio:['pipe','pipe','pipe']});
    serverExit=once(server,'close');server.stdout.on('data',b=>serverText+=b);server.stderr.on('data',b=>serverError+=b);
    for(let n=0;n<500;n++){const m=serverText.match(/http:\/\/127\.0\.0\.1:\d+\//);if(m){base=m[0];break;}if(server.exitCode!==null)throw Error(serverError+serverText);await delay(20);}assert.ok(base,'Packaged launcher URL');
    browser=spawn(chrome,['--headless=new','--disable-gpu','--no-first-run','--no-default-browser-check','--disable-background-networking','--disable-component-update','--disable-sync','--metrics-recording-only','--remote-debugging-address=127.0.0.1','--remote-debugging-port=0','--user-data-dir='+path.join(dir,'profile'),'about:blank'],{cwd:inputs.cwd,env,windowsHide:true,stdio:['ignore','ignore','pipe']});
    browserExit=once(browser,'close');browser.stderr.on('data',b=>browserText+=b);
    let endpoint;for(let n=0;n<500;n++){endpoint=browserText.match(/DevTools listening on (ws:\/\/[^\s]+)/)?.[1];if(endpoint)break;if(browser.exitCode!==null)throw Error('Browser exited: '+browserText);await delay(20);}assert.ok(endpoint);
    socket=new WebSocket(endpoint);await once(socket,'open');socket.addEventListener('message',event=>{const m=JSON.parse(event.data);if(m.id){const p=pending.get(m.id);pending.delete(m.id);if(m.error)p?.reject(Error(JSON.stringify(m.error)));else p?.resolve(m.result);}if(m.method==='Network.requestWillBeSent')requests.push(m.params.request.url);if(m.method==='Runtime.exceptionThrown')errors.push(m.params.exceptionDetails);});
    const send=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const id=++seq;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params,sessionId}));});
    const version=await send('Browser.getVersion');const {targetId}=await send('Target.createTarget',{url:'about:blank'});const {sessionId}=await send('Target.attachToTarget',{targetId,flatten:true});const cdp=(method,params)=>send(method,params,sessionId);
    await cdp('Page.enable');await cdp('Runtime.enable');await cdp('Network.enable');await cdp('Emulation.setDeviceMetricsOverride',{width:1440,height:1024,deviceScaleFactor:2,mobile:false});
    const evaluate=async expression=>{const r=await cdp('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true});if(r.exceptionDetails)throw Error(JSON.stringify(r.exceptionDetails));return r.result.value;};
    const until=expression=>evaluate(`new Promise((resolve,reject)=>{let n=0;const t=setInterval(()=>{if(${expression}){clearInterval(t);resolve(true)}else if(++n>500){clearInterval(t);reject(Error('Browser condition timed out'))}},10)})`);
    const check=async(label,expression)=>{assert.equal(await evaluate(expression),true,label);checks.push(label);};
    const screenshot=async label=>{const r=await cdp('Page.captureScreenshot',{format:'png',captureBeyondViewport:false});await fs.writeFile(path.join(dir,label+'.png'),Buffer.from(r.data,'base64'));};
    const key=async(key,shift=false)=>{for(const type of ['keyDown','keyUp'])await cdp('Input.dispatchKeyEvent',{type,key,code:key,windowsVirtualKeyCode:key==='Tab'?9:13,modifiers:shift?8:0,...(type==='keyDown'&&key==='Enter'?{text:'\r'}:{})});};
    await cdp('Page.navigate',{url:base});await until(name==='refused'?"document.querySelector('[role=alert]')":name==='static'?"document.querySelector('.projects-panel')":"document.querySelectorAll('.result').length>0");
    if(secondCatalog){
      await packageComparisonChecks({check,evaluate,cdp,screenshot,until,key,catalogs:[catalog,secondCatalog],scoped:name==='pair-scope'});
    }else if(name==='refused'){
      await check('Scope refusal is explicit, without static/empty success',"document.querySelector('[role=alert]').textContent.includes('Catalog load refused')&&!document.querySelector('.result')&&!document.body.textContent.includes('Photography NAS')");await screenshot('scope-refused');
    }else if(name==='static'){
      await check('Static Library is intentional synthetic presentation',"document.body.textContent.includes('Photography NAS')&&!document.querySelector('#catalog-identity')");
      await evaluate("document.querySelector('nav a[href=\"#find\"]').click()");await until("document.querySelector('h1').textContent==='Find'");await check('Static Find remains available',"document.querySelectorAll('.result').length>0");await screenshot('static-find');
    }else{
      await check('Catalog read-only notice',"document.querySelector('#scope').textContent.includes('No source/media files are opened')&&document.querySelector('#catalog-identity').textContent.includes('Session load time')");
      await check('Persisted scope disclosure',name==='foreign'?"document.querySelector('#inventory-scope').textContent.includes('UNKNOWN')":`document.querySelector('#inventory-scope').textContent.includes(${JSON.stringify('.DS_Store exclusion '+(name==='on'?'ON':'OFF'))})&&document.querySelector('#inventory-scope').textContent.includes(${JSON.stringify((name==='on'?2:0)+' excluded')})`);await screenshot('library');
      await evaluate("document.querySelector('nav a[href=\"#find\"]').click()");await until("document.querySelector('h1').textContent==='Find'");
      const exact=name==='foreign'?'line\nname.txt':"nested/O'Brien & +%# note.txt";
      await evaluate("document.querySelector('[aria-label=\"Exact name (JSON string)\"]').click()");await until("document.querySelector('[aria-label=\"Exact recorded name (JSON string)\"]')");
      await evaluate("document.querySelector('[aria-label=\"Exact recorded name (JSON string)\"]').focus()");await cdp('Input.dispatchKeyEvent',{type:'keyDown',key:'a',code:'KeyA',windowsVirtualKeyCode:65,modifiers:2});await cdp('Input.dispatchKeyEvent',{type:'keyUp',key:'a',code:'KeyA',windowsVirtualKeyCode:65,modifiers:2});await cdp('Input.insertText',{text:JSON.stringify(exact)});await key('Enter');await until("document.querySelectorAll('.result').length===1");await evaluate("document.querySelector('.result').click()");
      await check('Exact special name selects its inspector',name==='foreign'?"document.querySelector('.inspector').textContent.includes('File record 9007199254740993')&&document.querySelector('.inspector').textContent.includes('JSON name:')":`document.querySelector('.inspector').textContent.includes(${JSON.stringify(exact)})`);await screenshot('find-inspector');
      const state=()=>evaluate("({hash:location.hash,time:performance.timeOrigin,entry:document.querySelector('[aria-label=\"Exact recorded name (JSON string)\"]').value,selected:document.querySelector('.result[aria-pressed=true]').textContent,detail:document.querySelector('.inspector').textContent})");const before=await state();await evaluate("document.querySelector('nav a').focus()");await key('Tab',true);await check('Keyboard reaches visible skip',"document.activeElement.matches('.skip:focus-visible')");await key('Enter');await check('Skip focuses main and preserves query/selection',`document.activeElement===document.querySelector('main')&&${JSON.stringify(JSON.stringify(before)===JSON.stringify(await state()))}`);await key('Tab');await check('Tab enters workspace',"document.activeElement===document.querySelector('[data-mode=Guided]')");
      await evaluate("document.querySelector('[data-mode=Expert]').click()");await until("document.querySelector('.result[aria-pressed=true]')");assert.deepEqual(await state(),before);checks.push('Disclosure retains exact query and evidence');
    }
    assert.deepEqual(errors,[]);assert.ok(requests.every(url=>url==='about:blank'||url.startsWith(base)));checks.push('No observed external application requests or runtime exceptions');
    await fs.writeFile(path.join(dir,'browser.json'),JSON.stringify({version,checks,requests,errors,viewport:{width:1440,height:1024,scale:2}},null,2));
    await send('Browser.close');await browserExit;
  }catch(error){failed=error.stack;}
  finally{
    clearTimeout(guard);
    if(browser&&browser.exitCode===null&&browser.signalCode===null){forcedBrowser=true;browser.kill();await browserExit;}
    socket?.close();
    if(server&&server.exitCode===null){server.stdin.write('st');await delay(80);server.stdin.write('op\n');const timeout=setTimeout(()=>{forcedServer=true;server.kill();},10000);await serverExit;clearTimeout(timeout);}else if(serverExit)await serverExit;
    const result={name,package:pkg,catalog,secondCatalog,cwd:inputs.cwd,checks,durationMs:Date.now()-started,failed,forcedBrowser,forcedServer,serverPID:server?.pid,browserPID:browser?.pid,serverExit:server?.exitCode,browserExit:browser?.exitCode,serverText,serverError,browserText};summary.push(result);await fs.writeFile(path.join(evidence,'summary.json'),JSON.stringify(summary,null,2));
  }
  assert.equal(failed,undefined,failed);assert.equal(forcedServer,false);assert.equal(forcedBrowser,false);assert.equal(server.exitCode,0);assert.match(serverText,/Preview stopped \(stdin stop\)/);await assert.rejects(fetch(base));
  if(secondCatalog){const exits=JSON.parse(serverText.match(/catalog reader waited: (.+)/)[1]);assert.equal(exits.length,2);assert.ok(exits.every(e=>e.code===0&&e.signal===null&&Number.isInteger(e.pid)));}
}
console.log(JSON.stringify({sessions:summary.length,assertions:summary.map(x=>({name:x.name,checks:x.checks.length})),normalStopped:summary.length}));
