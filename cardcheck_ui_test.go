package main

// cardcheck_ui_test.go — the Card Check screen, run from the REAL ui/index.html
// script through the same harness as the job UI tests. The screen reports copies and
// dates and never gives a format verdict; a check that stops is reported with a retry.

import "testing"

const cardCheckUIAssertions = `
// ============================ assertions ====================================
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
const verdict=/safe[\s_-]*to|ok[\s_-]*to[\s_-]*format|\b(re)?format|\berase|\breuse|\bwipe/i;
const pad=n=>String(n).padStart(2,'0');
const local=iso=>{const d=new Date(iso);return pad(d.getMonth()+1)+'/'+pad(d.getDate())+'/'+d.getFullYear()+' '+pad(d.getHours())+':'+pad(d.getMinutes())+':'+pad(d.getSeconds())};
const clean={id:1,kind:'cardcheck',label:'Check card E:',status:'COMPLETED',progress:1,created_at:'2026-11-20T00:00:00Z',finished_at:'2026-11-20T00:00:05Z'};

(async()=>{
  const older='2026-11-20T18:42:07Z',newer='2026-11-22T18:42:07Z';
  const all={total_files:1204,total_bytes:10,backed_up_files:1204,backed_up_bytes:10,no_copy_files:0,fewest_copies:2,as_of:older,
    backed_up:[{rel:'DCIM/IMG_1.JPG',size:5,copies:[{where:'DRIVE-A',last_verified_at:older,offline:true},{where:'DRIVE-B',last_verified_at:newer}]}]};

  // ---- 1. everything has a copy: counts and the stalest date, no verdict --------
  ccRender(all,'E:',clean);
  let h=__el('#ccresult').innerHTML;
  ck(h.includes('1,204 of 1,204 files backed up as of '+local(older)+'. Fewest copies: 2.'),'the summary states the count, the oldest verified time, and the fewest copies; got: '+h);
  ck(!verdict.test(h),'the screen must not carry a format verdict; got: '+h);
  ck(h.includes('DRIVE-A')&&h.includes(local(newer)),'each copy shows where it lives and when it was last verified');
  ck(h.includes('offline, not rechecked'),'an offline copy is marked as not rechecked');

  // ---- 2. files with no copy are said first ------------------------------------
  ccRender({...all,backed_up_files:1167,no_copy_files:37,no_copy:[{rel:'IMG_9.JPG',size:1,recorded_in:'Archive: Trip',recorded_at:'2026-01-02T12:00:00Z'},{rel:'IMG_8.JPG',size:1}]},'E:',clean);
  h=__el('#ccresult').innerHTML;
  const a=h.indexOf('37 of 1,204 files have no backup copy.'),b=h.indexOf('1,167 of 1,204 files backed up as of');
  ck(a>=0&&b>a,'the no-copy count comes first; got: '+h);
  ck(h.includes('recorded on '+local('2026-01-02T12:00:00Z').slice(0,10))&&h.includes('no backup copy'),'a recorded-only file says when it was recorded and that it has no backup copy');
  ck(!verdict.test(h),'no verdict with missing copies either');

  // ---- 3. incomplete: hidden items and unreadable files --------------------------
  ccRender({...all,incomplete:true,excluded:2,excluded_items:[{path:'.private',folder:true},{path:'.x.jpg'}],skipped:1,problems:[{path:'E:/bad',kind:'hash',err:'io'}]},'E:',clean);
  h=__el('#ccresult').innerHTML;
  ck(h.includes('INCOMPLETE')&&h.includes('were not checked')&&h.includes('.private'),'excluded items make the screen say it is incomplete, and list them; got: '+h);
  ck(!verdict.test(h),'no verdict on an incomplete card');

  // ---- 4. empty card --------------------------------------------------------------
  ccRender({total_files:0},'E:',clean);
  ck(__el('#ccresult').innerHTML.includes('No files found'),'an empty card says so');

  // ---- 5. the poll ends on INTERRUPTED and offers a retry ------------------------
  __el('#ccresult').innerHTML='';
  __routes={'POST /api/card-check':{job_id:7},'GET /api/jobs':[{...clean,id:7,status:'INTERRUPTED',progress:0.3}]};
  await Promise.race([ccStart('E:'),new Promise(r=>setTimeout(r,4000))]);
  h=__el('#ccresult').innerHTML;
  ck(h.includes('interrupted')&&h.includes('Check again'),'an interrupted check ends the poll and offers a retry; got: '+h);

  // ---- 6. FAILED also offers a retry ----------------------------------------------
  __routes={'POST /api/card-check':{job_id:8},'GET /api/jobs':[{...clean,id:8,status:'FAILED',label:'Check card E: — ERROR: card removed'}]};
  await Promise.race([ccStart('E:'),new Promise(r=>setTimeout(r,4000))]);
  h=__el('#ccresult').innerHTML;
  ck(h.includes('card removed')&&h.includes('Check again'),'a failed check shows its error and a retry; got: '+h);

  // ---- 7. a finished but unrecorded check still shows its report, with the warning --
  __routes={'POST /api/card-check':{job_id:9},'GET /api/jobs':[{...clean,id:9,unrecorded:true,result:all}]};
  await Promise.race([ccStart('E:'),new Promise(r=>setTimeout(r,4000))]);
  h=__el('#ccresult').innerHTML;
  ck(h.includes(JOB_UNRECORDED_MSG)&&h.includes('files backed up as of'),'an unrecorded check keeps its report and carries the warning; got: '+h);

  if(__fail.length){console.error('FAIL\n - '+__fail.join('\n - '));process.exit(1)}
  console.log('PASS');process.exit(0);
})();
`

// TestCardCheckUI_ReportsCopiesNotAVerdict drives ui/index.html's own Card Check
// rendering and polling.
func TestCardCheckUI_ReportsCopiesNotAVerdict(t *testing.T) {
	runUIHarness(t, "cardcheck_ui_harness.js", cardCheckUIAssertions,
		"SKIPPED, NOT PASSED: node is not on PATH, so ui/index.html's Card Check report "+
			"and its polling were NOT executed here.")
}
