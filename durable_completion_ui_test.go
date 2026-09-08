package main

// durable_completion_ui_test.go — OB-002 / PR-03 follow-up, Blocker 2: the existing
// job UI must honour the recording-state contract.
//
// This runs the REAL script out of ui/index.html. The page's <script> block is
// extracted verbatim, sandwiched between a small browser stub and a block of
// assertions, and executed by the `node` that is already on this machine. Nothing is
// installed, no framework is added, and no part of the page's logic is restated here —
// a test that re-implemented jobStamp or waitJob would pass whatever the page did.
//
// The stub is deliberately thin: a fake element per selector, storage, a fetch seam,
// and silenced intervals. fetch hangs until a case installs a reply table, which parks
// the page's own boot-time calls without editing the page. setTimeout stays real
// because waitJob's poll depends on it.
//
// If node is unavailable the test SKIPS with the exact coverage that is then missing,
// rather than passing on the strength of having read the file.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// uiScript returns the contents of ui/index.html's single <script> block.
func uiScript(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("ui", "index.html"))
	if err != nil {
		t.Fatalf("read ui/index.html: %v", err)
	}
	s := string(b)
	open := strings.Index(s, "<script>")
	end := strings.LastIndex(s, "</script>")
	if open < 0 || end < 0 || end < open {
		t.Fatal("ui/index.html: could not find the <script> block")
	}
	return s[open+len("<script>") : end]
}

const uiPrelude = `
// --- minimal browser stubs (no framework, no dependency) ---------------------
const noop=()=>{};
function makeEl(){
  return {innerHTML:'',outerHTML:'',textContent:'',value:'',checked:false,className:'',style:{},dataset:{},
    classList:{add:noop,remove:noop,toggle:noop,contains:()=>false},
    children:[],parentNode:null,
    append:noop,appendChild:noop,removeChild:noop,remove:noop,insertAdjacentHTML:noop,
    addEventListener:noop,removeEventListener:noop,
    querySelector:()=>makeEl(),querySelectorAll:()=>[],closest:()=>null,
    setAttribute:noop,getAttribute:()=>null,removeAttribute:noop,hasAttribute:()=>false,
    focus:noop,blur:noop,click:noop,showModal:noop,close:noop,scrollIntoView:noop,
    getBoundingClientRect:()=>({width:0,height:0,top:0,left:0,right:0,bottom:0}),
    getContext:()=>null,select:noop,scrollTo:noop};
}
const _els=new Map();
function el(sel){if(!_els.has(sel))_els.set(sel,makeEl());return _els.get(sel);}
globalThis.__el=el;
globalThis.document={
  querySelector:el,querySelectorAll:()=>[],
  // Nothing is actually mounted, so an id lookup finds nothing — which is also what
  // keeps the perf strip's 2s poller from starting inside the job-detail view.
  getElementById:()=>null,
  createElement:()=>makeEl(),createTextNode:()=>makeEl(),
  addEventListener:noop,removeEventListener:noop,
  body:makeEl(),documentElement:makeEl(),activeElement:makeEl(),head:makeEl()};
function storage(){const m=new Map();return{getItem:k=>m.has(k)?m.get(k):null,setItem:(k,v)=>m.set(k,String(v)),removeItem:k=>m.delete(k),clear:()=>m.clear()};}
globalThis.localStorage=storage();
globalThis.sessionStorage=storage();
globalThis.location={hash:'',href:'http://localhost/',reload:noop};
globalThis.addEventListener=noop;globalThis.removeEventListener=noop;
globalThis.prompt=()=>'';globalThis.alert=noop;globalThis.confirm=()=>true;
globalThis.requestAnimationFrame=noop;
globalThis.window=globalThis;
// Intervals are silenced so the page's own 20s pollers cannot keep node alive or fire
// mid-assertion. setTimeout stays REAL — waitJob's sleep needs it.
globalThis.setInterval=()=>0;globalThis.clearInterval=noop;
// The one seam. Until a case installs a reply table every fetch hangs forever, which
// parks the page's boot-time api() calls without editing the page.
globalThis.__fetch=null;
globalThis.fetch=(u,o)=>globalThis.__fetch?globalThis.__fetch(u,o):new Promise(()=>{});
// Toasts are the evidence in one case, so they are recorded rather than dropped —
// through the page's own toast(), which appends a real element to #toast.
globalThis.__toasts=[];
el('#toast').append=d=>{globalThis.__toasts.push({text:d.textContent,bad:d.className==='bad'})};
`

const uiAssertions = `
// ============================ assertions ====================================
// Everything above this line is the real ui/index.html script, unmodified.
const __fail=[];
const ck=(cond,msg)=>{if(!cond)__fail.push(msg)};
process.on('unhandledRejection',e=>{__fail.push('unhandled rejection: '+((e&&e.message)||e))});

let __routes={};
globalThis.__fetch=async(u,o)=>{
  const method=(o&&o.method)||'GET';
  const body=__routes[method+' '+u];
  const out=body===undefined?{}:body;
  return {status:200,ok:true,statusText:'OK',json:async()=>out,text:async()=>JSON.stringify(out)};
};

const clean ={id:1,kind:'scan',label:'Scan clean',status:'COMPLETED',progress:1,created_at:'2026-09-07T00:00:00Z',finished_at:'2026-09-07T00:00:05Z',result:{files:2},artifacts:[{kind:'catalog',label:'2 records'}]};
const unrec ={...clean,id:2,label:'Scan unrecorded',unrecorded:true,persist_error:'jobs.json unwritable (injected)'};
const healed={...clean,id:3,label:'Scan healed',persist_error:'jobs.json unwritable (injected)'};

// ---- 1. the status helpers -------------------------------------------------
ck(jobStamp(clean)==='VERIFIED','positive control: a clean recorded completion must keep the VERIFIED stamp');
ck(jobStamp(unrec)!=='VERIFIED','an unrecorded completion must NOT render the VERIFIED success stamp');
ck(jobStamp(unrec)==='UNRECORDED','an unrecorded completion needs its own stamp class');
ck(jobStampText(unrec)==='NOT RECORDED','the stamp must not read a bare COMPLETED for an unrecorded job');
ck(jobStampText(clean)==='COMPLETED','positive control: a clean job still reads COMPLETED');
ck(jobStamp(healed)==='VERIFIED','a job a later save recorded is a clean success again - history is not a current warning');
ck(jobStampText(healed)==='COMPLETED','a recorded job must not keep saying NOT RECORDED');
ck(jobRecordingNote(clean)==='','positive control: a clean job carries no recording note');
ck(jobRecordingNote(unrec).includes(JOB_UNRECORDED_MSG),'the unrecorded warning must be shown in plain words');
ck(jobRecordingNote(unrec).includes('jobs.json unwritable'),'the cause must be shown with the warning');
ck(jobRecordingNote(healed)!=='','an earlier failure must stay available as diagnostic history');
ck(!jobRecordingNote(healed).includes(JOB_UNRECORDED_MSG),'a recorded job must not carry the not-recorded warning');
ck(jobRecordingNote(healed).includes('Recorded.'),'the history note must say the record is present');

(async()=>{
  // ---- 2. the real jobs list ----------------------------------------------
  __routes={'GET /api/jobs':[unrec],'GET /api/config':{config:{}}};
  await vJobs();
  let h=__el('#view').innerHTML;
  ck(!/stamp VERIFIED/.test(h),'the Jobs list rendered a VERIFIED stamp for an unrecorded completion');
  ck(/stamp UNRECORDED/.test(h),'the Jobs list must render the unrecorded stamp');
  ck(h.includes(JOB_UNRECORDED_MSG),'the Jobs list must show the explicit warning');
  ck(h.includes('2 file(s) cataloged'),'the result must stay visible alongside the warning');

  __routes={'GET /api/jobs':[clean],'GET /api/config':{config:{}}};
  await vJobs();
  h=__el('#view').innerHTML;
  ck(/stamp VERIFIED/.test(h),'positive control: a clean completion must still render VERIFIED');
  ck(!h.includes(JOB_UNRECORDED_MSG),'positive control: a clean completion must carry no warning');

  // ---- 3. the real job-detail view ----------------------------------------
  __routes={'GET /api/jobs/2':unrec,'GET /api/config':{config:{}}};
  await vJobDetail(2);
  h=__el('#view').innerHTML;
  ck(!/stamp VERIFIED/.test(h),'the job detail rendered a VERIFIED stamp for an unrecorded completion');
  ck(h.includes(JOB_UNRECORDED_MSG),'the job detail must show the explicit warning');
  ck(h.includes('2 records'),'the artifacts must stay reachable with the warning');

  __routes={'GET /api/jobs/3':healed,'GET /api/config':{config:{}}};
  await vJobDetail(3);
  h=__el('#view').innerHTML;
  ck(/stamp VERIFIED/.test(h),'a recorded job must read as a clean success again');
  ck(!h.includes(JOB_UNRECORDED_MSG),'a recorded job must stop claiming its record is absent');
  ck(h.includes('An earlier attempt to record this job failed'),'the earlier failure must remain as history');

  // ---- 4. waitJob ----------------------------------------------------------
  __routes={'GET /api/jobs':[clean]};
  const okJob=await waitJob(1);
  ck(okJob&&okJob.id===1,'positive control: waitJob must resolve for a clean recorded completion');

  __routes={'GET /api/jobs':[unrec]};
  let rejected=null;
  try{await waitJob(2);ck(false,'waitJob resolved as clean success for an unrecorded completion')}
  catch(e){rejected=e}
  ck(rejected&&rejected.unrecorded===true,'waitJob must take a distinct warning path, not the ordinary error path');
  ck(rejected&&rejected.job&&rejected.job.result,'the warning must carry the job, so callers keep access to the result');
  ck(rejected&&rejected.message.includes(JOB_UNRECORDED_MSG),'the rejection must say what happened in plain words');

  __routes={'GET /api/jobs':[{...clean,id:4,status:'INTERRUPTED'}]};
  let interrupted=false;
  try{await waitJob(4)}catch(e){interrupted=true}
  ck(interrupted,'waitJob must stop on a terminal INTERRUPTED instead of polling forever');

  // ---- 5. a real caller: no ordinary success toast -------------------------
  __toasts.length=0;
  const planPayload={plan:{id:9,name:'P',status:'READY',destination_root:'D:'},report:{compilable:true},unrouted:[],coverage:{}};
  __routes={'POST /api/plans/9/adopt-destination':{job_id:2},'GET /api/jobs':[unrec],'GET /api/plans/9':planPayload};
  adoptDest(9);
  await new Promise(r=>setTimeout(r,1200));
  let texts=__toasts.map(t=>t.text).join(' | ');
  ck(!texts.includes('Destination adopted'),'an unrecorded completion must not trigger the ordinary success toast; got: '+texts);
  ck(texts.includes(JOB_UNRECORDED_MSG),'the caller must surface the warning; got: '+texts);
  ck(__toasts.some(t=>t.bad),'the warning must use the warning/error toast path, not the plain one');

  __toasts.length=0;
  __routes={'POST /api/plans/9/adopt-destination':{job_id:1},'GET /api/jobs':[clean],'GET /api/plans/9':planPayload};
  adoptDest(9);
  await new Promise(r=>setTimeout(r,1200));
  texts=__toasts.map(t=>t.text).join(' | ');
  ck(texts.includes('Destination adopted'),'positive control: a clean completion must still toast success; got: '+texts);

  // ---- 6. the card-check consumer -----------------------------------------
  ccRender({safe_to_format:true,total_files:1,total_bytes:10},'E:',unrec);
  const cc=__el('#ccresult').innerHTML;
  ck(cc.includes(JOB_UNRECORDED_MSG),'the card-check result must carry the warning when its job was not recorded');
  ck(cc.includes('Safe to format'),'the card-check verdict itself must not be discarded');
  ccRender({safe_to_format:true,total_files:1,total_bytes:10},'E:',clean);
  ck(!__el('#ccresult').innerHTML.includes(JOB_UNRECORDED_MSG),'positive control: a recorded card check carries no warning');

  if(__fail.length){console.error('FAIL\n - '+__fail.join('\n - '));process.exit(1)}
  console.log('PASS');process.exit(0);
})();
`

// uiPrecedenceAssertions covers execution-outcome-vs-recording-state precedence. It is a
// second assertion block rather than an extension of the first so that a red run against
// the pre-edit page isolates exactly the reported regression instead of mixing it with
// the already-passing COMPLETED cases.
const uiPrecedenceAssertions = `
// ============================ assertions ====================================
// Everything above this line is the real ui/index.html script, unmodified.
const __fail=[];
const ck=(cond,msg)=>{if(!cond)__fail.push(msg)};
process.on('unhandledRejection',e=>{__fail.push('unhandled rejection: '+((e&&e.message)||e))});

let __routes={}, __jobsHits=0;
globalThis.__fetch=async(u,o)=>{
  const method=(o&&o.method)||'GET';
  if(method==='GET'&&u==='/api/jobs')__jobsHits++;
  const body=__routes[method+' '+u];
  const out=body===undefined?{}:body;
  return {status:200,ok:true,statusText:'OK',json:async()=>out,text:async()=>JSON.stringify(out)};
};

const base   ={kind:'scan',progress:1,created_at:'2026-09-07T00:00:00Z',finished_at:'2026-09-07T00:00:05Z'};
// A. the ordinary failure: the operation failed, and that failure WAS recorded.
const failed ={...base,id:11,label:'Scan the vault — ERROR: source unreadable',status:'FAILED'};
// B. the combined failure: the operation failed AND its failure record could not be
//    written. This is reachable in production — main.go finishes a failing job with
//    FinishJob(..., "FAILED", nil, nil), and a failed sidecar write flags ANY terminal
//    status, FAILED included.
const failedUnrec={...failed,id:12,unrecorded:true,persist_error:'jobs.json unwritable (injected)'};
// C. the completed-but-unrecorded job the earlier block already covers, kept here as the
//    contrast case: this one MAY say the work finished, because it did.
const doneUnrec={...base,id:13,label:'Scan clean',status:'COMPLETED',unrecorded:true,persist_error:'jobs.json unwritable (injected)',result:{files:2},artifacts:[{kind:'catalog',label:'2 records'}]};
// D. recorded now, with an earlier persistence failure kept only as history.
const healed ={...base,id:14,label:'Scan healed',status:'COMPLETED',persist_error:'jobs.json unwritable (injected)',result:{files:2},artifacts:[{kind:'catalog',label:'2 records'}]};
// E. INTERRUPTED, as the application actually produces it: loadJobs sets this status on
//    restart and deliberately does NOT flag those rows, so INTERRUPTED is never
//    unrecorded in production.
const interrupted={...base,id:15,label:'Scan interrupted',status:'INTERRUPTED',progress:0.4,finished_at:''};
// SYNTHETIC, not a backend state: no code path produces INTERRUPTED + unrecorded. It is
// supplied only to prove the helpers degrade defensively if one ever appeared.
const interruptedUnrecSynthetic={...interrupted,id:16,unrecorded:true,persist_error:'jobs.json unwritable (injected)'};

const claimsSuccess=s=>/results are real|operation itself finished|Work finished/.test(s);

// ---- A. an ordinary FAILED job is untouched --------------------------------
ck(jobStamp(failed)==='FAILED','control: a recorded failure must keep the FAILED stamp');
ck(jobStampText(failed)==='FAILED','control: a recorded failure must read FAILED');
ck(jobRecordingNote(failed)==='','control: a recorded failure carries no recording note');

// ---- B. FAILED + unrecorded: the failure is still the headline -------------
ck(jobStamp(failedUnrec)==='FAILED','a FAILED job whose record was not saved is still a FAILED job - the recording state must not displace the execution outcome');
ck(jobStamp(failedUnrec)!=='UNRECORDED','the recording qualification must not replace the failure stamp or its failure styling');
ck(jobStampText(failedUnrec)==='FAILED','the stamp must still read FAILED, not NOT RECORDED');
const bNote=jobRecordingNote(failedUnrec);
ck(bNote!=='','the recording failure must still be shown as a distinct qualification');
ck(!claimsSuccess(bNote),'the note for a FAILED job must not assert that the work finished or that its results are real; got: '+bNote);
ck(!/results are (real|valid|usable)|still available here/.test(bNote),'the note must not claim a failed job produced usable results; got: '+bNote);
ck(/failed/i.test(bNote),'the note must name the failure as the primary outcome; got: '+bNote);
ck(bNote.includes('failure record could not be saved'),'the note must say it is the FAILURE RECORD that could not be saved; got: '+bNote);
ck(bNote.includes('jobs.json unwritable'),'the recording cause must stay visible as a distinct qualification');

// ---- C. COMPLETED + unrecorded keeps its own, different sentence -----------
const cNote=jobRecordingNote(doneUnrec);
ck(jobStamp(doneUnrec)==='UNRECORDED','a completed-but-unrecorded job keeps its own stamp');
ck(jobStampText(doneUnrec)==='NOT RECORDED','a completed-but-unrecorded job must not read a bare COMPLETED');
ck(cNote.includes(JOB_UNRECORDED_MSG),'the completed-but-unrecorded warning must survive this change');
ck(cNote!==bNote,'a failed job and a completed job must not share one recording sentence');
// Completion is not verification. The note may DENY verification (it does: "is not
// verified history"); what it must never do is assert that reaching COMPLETED proves the
// written content was read back and checked.
ck(!/contents? (were |was )?verified|verified against|checked the (files|bytes|contents)|read back and (checked|verified)/i.test(cNote),'completion is not verification: the note must not claim file content was checked; got: '+cNote);
ck(/not verified history/i.test(cNote),'the completed-but-unrecorded note must keep saying this completion is not verified history');

// ---- D. recorded now, earlier failure is history only ----------------------
ck(jobStamp(healed)==='VERIFIED','a job a later save recorded is a clean success again');
ck(jobStampText(healed)==='COMPLETED','a recorded job must not be labelled NOT RECORDED');
ck(!jobRecordingNote(healed).includes(JOB_UNRECORDED_MSG),'history is not a current warning');
ck(jobRecordingNote(healed).includes('Recorded.'),'the history note must say the record is present now');

// ---- E. INTERRUPTED stays INTERRUPTED --------------------------------------
ck(jobStamp(interrupted)==='INTERRUPTED','an interrupted job keeps the INTERRUPTED stamp');
ck(jobStampText(interrupted)==='INTERRUPTED','an interrupted job must read INTERRUPTED');
ck(jobRecordingNote(interrupted)==='','a plain interrupted job carries no recording note');
// synthetic only - asserted as defensive degradation, not as an application behaviour
ck(jobStamp(interruptedUnrecSynthetic)==='INTERRUPTED','SYNTHETIC: if the combination ever appeared, the interruption must still be the outcome');
ck(jobStampText(interruptedUnrecSynthetic)==='INTERRUPTED','SYNTHETIC: the stamp must not read NOT RECORDED for an interrupted job');
ck(!claimsSuccess(jobRecordingNote(interruptedUnrecSynthetic)),'SYNTHETIC: an interrupted job must never be described as finished with real results');

(async()=>{
  const cfgRoute={'GET /api/config':{config:{}}};

  // ---- B, composed: the actual jobs-list output ----------------------------
  __routes={'GET /api/jobs':[failedUnrec],...cfgRoute};
  await vJobs();
  let h=__el('#view').innerHTML;
  ck(/stamp FAILED/.test(h),'the Jobs list must render the FAILED stamp for a failure whose record was not saved');
  ck(!/stamp UNRECORDED/.test(h),'the Jobs list must not swap the failure stamp for the unrecorded stamp');
  ck(!/stamp VERIFIED/.test(h),'the Jobs list must never render a success stamp for a failed job');
  ck(h.includes('FAILED'),'the word FAILED must appear in the composed jobs list');
  ck(!h.includes('NOT RECORDED'),'the primary stamp text must stay FAILED, not NOT RECORDED');
  ck(h.includes('ERROR: source unreadable'),'the original operation error must remain available in the list');
  ck(h.includes('failure record could not be saved'),'the recording failure must appear as an additional qualification');
  ck(!claimsSuccess(h),'the composed list must not claim a failed job finished with real results');

  // ---- B, composed: job detail --------------------------------------------
  __routes={'GET /api/jobs/12':failedUnrec,...cfgRoute};
  await vJobDetail(12);
  h=__el('#view').innerHTML;
  ck(/stamp FAILED/.test(h),'the job detail must render the FAILED stamp for a failure whose record was not saved');
  ck(!/stamp UNRECORDED/.test(h),'the job detail must not swap the failure stamp for the unrecorded stamp');
  ck(!h.includes('NOT RECORDED'),'the job detail stamp must stay FAILED');
  ck(h.includes('ERROR: source unreadable'),'the original operation error must remain available in the detail view');
  ck(h.includes('failure record could not be saved'),'the detail view must carry the recording qualification too');
  ck(!claimsSuccess(h),'the composed detail view must not claim a failed job finished with real results');

  // ---- A, composed: an ordinary failure is unchanged -----------------------
  __routes={'GET /api/jobs':[failed],...cfgRoute};
  await vJobs();
  h=__el('#view').innerHTML;
  ck(/stamp FAILED/.test(h),'control: an ordinary failure still renders FAILED');
  ck(!h.includes('could not be saved'),'control: an ordinary failure carries no recording qualification');

  // ---- D, composed: no false current NOT RECORDED --------------------------
  __routes={'GET /api/jobs':[healed],...cfgRoute};
  await vJobs();
  h=__el('#view').innerHTML;
  ck(!h.includes('NOT RECORDED'),'a currently-recorded job must not be labelled NOT RECORDED by its history');
  ck(/stamp VERIFIED/.test(h),'a currently-recorded job reads as a clean success');
  ck(h.includes('An earlier attempt to record this job failed'),'the earlier failure stays available as history');

  // ---- E, composed: terminal presentation and polling ----------------------
  __routes={'GET /api/jobs':[interrupted],...cfgRoute};
  await vJobs();
  h=__el('#view').innerHTML;
  ck(/stamp INTERRUPTED/.test(h),'the Jobs list must render INTERRUPTED as the outcome');
  let hits=__jobsHits;
  await new Promise(r=>setTimeout(r,1400));
  ck(__jobsHits===hits,'a terminal INTERRUPTED job must not keep the jobs list re-polling');

  // ---- B: waitJob and its consumers ---------------------------------------
  __routes={'GET /api/jobs':[failedUnrec]};
  let rej=null;
  try{await waitJob(12);ck(false,'waitJob must not resolve for a FAILED job')}
  catch(e){rej=e}
  ck(rej,'waitJob must reject on a FAILED job whose record was not saved');
  ck(rej&&rej.unrecorded!==true,'a FAILED job must not take the completed-but-unrecorded warning path, which tells callers to show results');
  ck(rej&&rej.message.includes('source unreadable'),'the original operation problem must remain in the rejection; got: '+(rej&&rej.message));
  ck(rej&&/record could not be saved/.test(rej.message),'the recording qualification must remain available on the rejection; got: '+(rej&&rej.message));
  ck(rej&&rej.recordUnsaved===true,'the rejection must carry the recording qualification on its own distinct field');
  ck(rej&&rej.job&&rej.job.id===12,'the rejection must carry the job so callers can show the failure detail');

  // A: the ordinary failure rejection is unchanged
  __routes={'GET /api/jobs':[failed]};
  let rejA=null;
  try{await waitJob(11)}catch(e){rejA=e}
  ck(rejA&&rejA.message.includes('source unreadable'),'control: an ordinary failure still rejects with its own error');
  ck(rejA&&rejA.recordUnsaved!==true,'control: an ordinary failure carries no recording qualification');
  ck(rejA&&!/could not be saved/.test(rejA.message),'control: an ordinary failure message is not extended');

  // waitJob stops on terminal states, and nothing is retried or re-run.
  hits=__jobsHits;
  __routes={'GET /api/jobs':[interrupted]};
  let rejE=false;
  try{await waitJob(15)}catch(e){rejE=true}
  ck(rejE,'waitJob must stop on a terminal INTERRUPTED instead of polling forever');
  ck(__jobsHits-hits===1,'a terminal state must be decided on the first poll - no retry, no re-run');

  // ---- B and C through a real caller: no ordinary success toast ------------
  const planPayload={plan:{id:9,name:'P',status:'READY',destination_root:'D:'},report:{compilable:true},unrouted:[],coverage:{}};
  __toasts.length=0;
  __routes={'POST /api/plans/9/adopt-destination':{job_id:12},'GET /api/jobs':[failedUnrec],'GET /api/plans/9':planPayload};
  adoptDest(9);
  await new Promise(r=>setTimeout(r,1200));
  let texts=__toasts.map(t=>t.text).join(' | ');
  ck(!texts.includes('Destination adopted'),'a FAILED job whose record was not saved must not produce an ordinary success toast; got: '+texts);
  ck(!claimsSuccess(texts),'no notification may say a failed job finished with real results; got: '+texts);
  ck(texts.includes('source unreadable'),'the operation error must reach the user; got: '+texts);
  ck(__toasts.length>0&&__toasts.every(t=>t.bad),'the failure must use the warning/error toast path only; got: '+JSON.stringify(__toasts));

  __toasts.length=0;
  __routes={'POST /api/plans/9/adopt-destination':{job_id:13},'GET /api/jobs':[doneUnrec],'GET /api/plans/9':planPayload};
  adoptDest(9);
  await new Promise(r=>setTimeout(r,1200));
  texts=__toasts.map(t=>t.text).join(' | ');
  ck(!texts.includes('Destination adopted'),'a completed-but-unrecorded job must still not produce a clean-success toast; got: '+texts);
  ck(texts.includes(JOB_UNRECORDED_MSG),'the completed-but-unrecorded warning must still reach the user; got: '+texts);

  if(__fail.length){console.error('FAIL\n - '+__fail.join('\n - '));process.exit(1)}
  console.log('PASS');process.exit(0);
})();
`

// runUIHarness assembles the browser stub, the REAL ui/index.html script and one block
// of assertions into a single file and runs it under the node already on this machine.
// Both UI tests go through here so neither can drift onto a different page or a
// different stub. skipMsg names the coverage that is lost when node is missing.
func runUIHarness(t *testing.T, name, assertions, skipMsg string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip(skipMsg)
	}
	harness := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(harness, []byte(uiPrelude+"\n"+uiScript(t)+"\n"+assertions), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, harness).CombinedOutput()
	if err != nil {
		t.Fatalf("ui/index.html job consumers do not honour the recording-state contract:\n%s", out)
	}
	if !strings.Contains(string(out), "PASS") {
		t.Fatalf("harness produced no verdict:\n%s", out)
	}
}

// TestJobsUI_HonoursRecordingState drives ui/index.html's own job rendering, waitJob
// and job consumers against a finished-but-unrecorded job, a recorded one, and a
// recorded one that failed to record earlier.
func TestJobsUI_HonoursRecordingState(t *testing.T) {
	runUIHarness(t, "jobs_ui_harness.js", uiAssertions,
		"SKIPPED, NOT PASSED: node is not on PATH, so ui/index.html's job list, "+
			"job detail, waitJob, adoptDest and the card-check consumer were NOT executed "+
			"here. Backend coverage is unaffected; the UI half of Blocker 2 is unverified "+
			"in this environment.")
}

// TestJobsUI_ExecutionOutcomeTakesPrecedence is the regression guard for the defect the
// focused recheck found: the recording qualification had been allowed to DISPLACE the
// execution outcome, so a FAILED job whose failure record also could not be written
// rendered as an amber "NOT RECORDED" stamp under a sentence claiming the operation had
// finished and its results were real.
//
// The rule this pins down: the execution outcome is PRIMARY and always survives; the
// recording state is an ADDITIONAL qualification that never overwrites it and never
// upgrades a failure into a completion.
func TestJobsUI_ExecutionOutcomeTakesPrecedence(t *testing.T) {
	runUIHarness(t, "jobs_ui_precedence_harness.js", uiPrecedenceAssertions,
		"SKIPPED, NOT PASSED: node is not on PATH, so the FAILED-plus-unrecorded status "+
			"precedence in ui/index.html's job list, job detail, jobStamp/jobStampText/"+
			"jobRecordingNote and waitJob was NOT executed here. Backend coverage is "+
			"unaffected; this regression is unverified in this environment.")
}
