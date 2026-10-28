package main

// destructive_ui_test.go — the delete gate's confirm dialog in ui/index.html, run
// for real under node through the same harness as the job UI tests.

import "testing"

const uiDestructiveAssertions = `
// ============================ assertions ====================================
const __fail=[];
const ck=(cond,msg)=>{if(!cond)__fail.push(msg)};
process.on('unhandledRejection',e=>{__fail.push('unhandled rejection: '+((e&&e.message)||e))});
const resp=(status,body)=>({status,ok:status<300,statusText:'',json:async()=>body,text:async()=>JSON.stringify(body)});
const waitFor=async(cond)=>{for(let i=0;i<200&&!cond();i++)await new Promise(r=>setTimeout(r,5));ck(cond(),'timed out waiting');};

// The dialog: its parts persist, and close() fires onclose as a browser does
// (Esc on a modal dialog closes it).
const d=__el('#ddestructive');
const parts={'#ddword':{value:''},'#ddok':{},'#ddno':{}};
d.querySelector=s=>parts[s]||{};
let shown=0;d.showModal=()=>{shown++};
d.close=function(){if(this.onclose)this.onclose()};

let execCalls=0,release=null;
globalThis.__fetch=async(u,o)=>{
  if(u==='/api/destructive/prepare')return resp(200,{token:'t',word:'Delete',title:'Quarantine p',targets:[{kind:'file',path:'p'}]});
  if(u==='/api/destructive/execute'){
    execCalls++;
    if(execCalls>1)return resp(410,{error:'this confirmation is unknown or was already used'});
    await new Promise(r=>{release=r});
    return resp(200,{ok:true,done:'the action ran'});
  }
  return resp(200,{});
};

(async()=>{
  const pr=confirmDestructive('quarantine',{path:'p'});
  await waitFor(()=>shown===1);
  parts['#ddword'].value='Delete';
  parts['#ddok'].onclick();          // confirm
  await waitFor(()=>execCalls===1);  // execute is now running
  d.close();                         // the user presses Esc
  parts['#ddok'].onclick();          // and clicks again
  await new Promise(r=>setTimeout(r,20));
  release();                         // the action finishes on the server
  let r=null;
  try{r=await pr}catch(e){ck(false,'the dialog reported a failure although the action ran: '+e.message)}
  ck(r&&r.done==='the action ran','the action ran, so the dialog must resolve with its result, not '+JSON.stringify(r));
  ck(execCalls===1,'a click while the action was running sent another execute ('+execCalls+' in all)');
  ck(!__toasts.some(t=>t.bad),'an error was shown although the action ran: '+JSON.stringify(__toasts));

  // Positive control: Cancel, or Esc, before confirming still means nothing ran.
  for(const how of ['cancel','esc']){
    shown=0;
    const pc=confirmDestructive('quarantine',{path:'p'});
    await waitFor(()=>shown===1);
    if(how==='cancel')parts['#ddno'].onclick();else d.close();
    ck((await pc)===null,how+' before confirming must resolve with nothing done');
  }
  ck(execCalls===1,'backing out sent an execute ('+execCalls+' in all)');
  console.log(__fail.length?('FAIL\n'+__fail.join('\n')):'PASS');
  process.exit(__fail.length?1:0);
})();
`

// TestDestructiveUI_InFlightConfirmation: once the typed word is confirmed and the
// action is running, closing the dialog or clicking again must not report that
// nothing happened, nor send a second execute.
func TestDestructiveUI_InFlightConfirmation(t *testing.T) {
	runUIHarness(t, "destructive_ui_harness.js", uiDestructiveAssertions,
		"SKIPPED, NOT PASSED: node is not on PATH, so ui/index.html's delete-gate dialog "+
			"was NOT executed here.")
}
