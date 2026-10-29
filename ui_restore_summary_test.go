package main

import "testing"

// When a restore keeps the current keystore paths (the restored keystores would
// share a device), the summary says so with the restore's note, and does not
// claim the keystores were restored into use.
const uiRestoreSummaryAssertions = `
const __fail=[];
const ck=(cond,msg)=>{if(!cond)__fail.push(msg)};
const base={archives:1,files:2,volumes:1,packages:1,jobs:0,verify_events:0,pre_restore_dir:'C:/pre'};
appRestoreSummary({...base,included_keys:true,keystore_note:'Kept your keystore paths <b>x</b>'});
let h=__el('#dlg').innerHTML;
ck(h.includes('Kept your keystore paths &lt;b&gt;x&lt;/b&gt;'),'the keystore note is not shown (escaped): '+h);
ck(!h.includes('included and restored'),'the summary still says the keystores were restored into use');
appRestoreSummary({...base,included_keys:true});
h=__el('#dlg').innerHTML;
ck(h.includes('included and restored'),'positive control: a restore that repointed the keystores says so');
console.log(__fail.length?('FAIL\n'+__fail.join('\n')):'PASS');
process.exit(__fail.length?1:0);
`

func TestUI_RestoreSummaryShowsKeystoreNote(t *testing.T) {
	runUIHarness(t, "ui_restore_summary_harness.js", uiRestoreSummaryAssertions,
		"SKIPPED, NOT PASSED: node is not on PATH, so the restore summary in ui/index.html was NOT executed here.")
}
