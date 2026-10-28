package main

// ui_handlers_test.go — the legacy UI never puts data into inline JavaScript.

import (
	"regexp"
	"testing"
)

// No inline event handler in ui/index.html interpolates escaped data: esc()
// makes text safe for HTML, not for a JavaScript string inside an attribute
// (the browser decodes the attribute first, and esc leaves ' alone), so a name
// holding a quote could run code. Data travels in data-args instead.
func TestUI_NoInlineHandlerInterpolatesData(t *testing.T) {
	script := uiScript(t)
	bad := regexp.MustCompile(`on[a-z]+="[^"]*\$\{esc\(|on[a-z]+='[^']*\$\{esc\(`).FindAllString(script, -1)
	if len(bad) > 0 {
		t.Fatalf("%d inline handler(s) still interpolate esc() into JavaScript, e.g. %q", len(bad), bad[0])
	}
}

const uiDispatchAssertions = `
const __fail=[];
const ck=(cond,msg)=>{if(!cond)__fail.push(msg)};
let got=null;
globalThis.__probe=(...a)=>{got=a};
UI_CALLS.add('__probe');
const hostile="x');alert(1);('\"<b>&";
const fakeEl=(attrs,tag)=>{const e={tagName:tag||'BUTTON',getAttribute:k=>attrs[k]===undefined?null:attrs[k]};e.closest=()=>e;return e};
// The attribute value the browser hands back is the JSON itself.
const decoded=s=>s.replace(/&quot;/g,'"').replace(/&lt;/g,'<').replace(/&gt;/g,'>').replace(/&amp;/g,'&');
const attr=callArgs(hostile,7,'');
ck(!/"/.test(attr),'callArgs output can end its attribute: '+attr);
_uiDispatch('data-call',{target:fakeEl({'data-call':'__probe','data-args':decoded(attr)})});
ck(JSON.stringify(got)===JSON.stringify([hostile,7,'']),'the call did not receive its arguments exactly: '+JSON.stringify(got));
// Only listed functions are called.
let evil=false;globalThis.__notListed=()=>{evil=true};
_uiDispatch('data-call',{target:fakeEl({'data-call':'__notListed','data-args':'[]'})});
ck(!evil,'a function not in UI_CALLS was called');
// A link's navigation is cancelled.
let prevented=false;
_uiDispatch('data-call',{target:fakeEl({'data-call':'__probe','data-args':'[1]'},'A'),preventDefault:()=>{prevented=true}});
ck(prevented,'a data-call link still navigates');
console.log(__fail.length?('FAIL\n'+__fail.join('\n')):'PASS');
process.exit(__fail.length?1:0);
`

// TestUI_DelegatedHandlersPassDataExactly runs the page's dispatcher: hostile
// text reaches the function as plain data, and only listed functions run.
func TestUI_DelegatedHandlersPassDataExactly(t *testing.T) {
	runUIHarness(t, "ui_dispatch_harness.js", uiDispatchAssertions,
		"SKIPPED, NOT PASSED: node is not on PATH, so ui/index.html's delegated handlers were NOT executed here.")
}
