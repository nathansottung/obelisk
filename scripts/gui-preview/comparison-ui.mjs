import { comparisonFrame, classes, validateComparison } from './recorded-comparison.mjs';
import { displayName, parseExactName } from './catalog-names.mjs';
let epoch=0;
export const cancelComparison=()=>{epoch++;};
const node=(tag,text,attrs={})=>{const n=document.createElement(tag);if(text!==undefined)n.textContent=text;for(const [k,v]of Object.entries(attrs))n.setAttribute(k,v);return n;};
export function renderComparison(content,snapshots,label) {
  const panel=node('section',undefined,{class:'panel',id:'recorded-comparison','aria-label':'Recorded comparison'});
  panel.append(node('p','Recorded snapshots only — no source files were opened or reverified. Results describe the selected recorded inventories and their scope, not current drive contents or backup health.',{class:'notice',id:'comparison-notice'}));
  const back=node('p');back.append(node('a','Return to Library',{href:'#library'}),node('span',' · '),node('a','Return to Find',{href:'#find'}));panel.append(back);
  const roots=node('div',undefined,{id:'comparison-roots'});
  for(const s of snapshots)roots.append(node('p',`${label(s)} — Recorded root: ${displayName(s.catalog.comparisonFrame?.root??'Unsupported or ambiguous')}. Recorded inventory time: ${s.catalog.recordedAt??'Unknown'}. Scope is shown in this source’s card above.`));
  roots.append(node('p','Proposed alignment: each recorded root’s relative path "" aligns with the other root’s relative path "". Slash is the only separator; internal backslashes are literal. No case folding, Unicode normalization, physical equivalence or suffix mapping is inferred.'));
  panel.append(roots);
  const choose=node('label','Comparison reference');
  const reference=node('select',undefined,{'aria-label':'Comparison reference',id:'comparison-reference'});
  reference.append(node('option','Choose a reference explicitly',{value:''}));
  for(const s of snapshots)reference.append(node('option',label(s),{value:s.handle}));choose.append(reference);panel.append(choose);
  const run=node('button','Accept root alignment and compare recorded snapshots',{id:'comparison-run',disabled:''});panel.append(run);
  let frameError='';try{snapshots.forEach(comparisonFrame);}catch(e){frameError=e.message;panel.append(node('p',frameError,{role:'alert',class:'error'}));}
  const filterLabel=node('label','Filter recorded paths (literal, case-sensitive)');
  const filter=node('input',undefined,{type:'search','aria-label':'Filter comparison paths',maxlength:'24578'});filterLabel.append(filter);
  const exactLabel=node('label','Exact path as JSON string');const exact=node('input',undefined,{type:'checkbox','aria-label':'Exact comparison path'});exactLabel.prepend(exact);
  const kindLabel=node('label','Recorded class');const kind=node('select',undefined,{'aria-label':'Recorded comparison class'});kind.append(node('option','All recorded classes',{value:''}));for(const [k,v]of Object.entries(classes))kind.append(node('option',v,{value:k}));kindLabel.append(kind);
  panel.append(filterLabel,exactLabel,kindLabel);
  const status=node('p','Choose a reference, inspect both roots/scopes, then accept the alignment.',{role:'status',id:'comparison-status'});
  const results=node('section',undefined,{'aria-label':'Comparison results',id:'comparison-results'});
  const details=node('section',undefined,{class:'panel',id:'comparison-detail','aria-label':'Two-sided recorded evidence'});
  panel.append(status,results,details);content.append(panel);
  let result=null;
  const clear=()=>{results.replaceChildren();details.replaceChildren();};
  const showDetail=row=>{
    details.replaceChildren(node('h2',displayName(row.key)),node('p',row.reason),node('p',row.scopeNote));
    details.dataset.request=result.request;
    for(const [side,handle,id]of [['Reference',result.reference,row.referenceId],['Counterpart',result.counterpart,row.counterpartId]]) {
      const snapshot=snapshots.find(s=>s.handle===handle),catalog=snapshot.catalog;
      const section=node('section',undefined,{'data-side':side.toLowerCase(),'data-snapshot':handle});
      section.append(node('h3',`${side}: ${label(snapshot)}`),node('p',`Artifact ${catalog.digest} · Session ${handle} · Recorded observation: ${catalog.recordedAt??'Unknown'}`),node('p',`Scope: ${catalog.inventoryScope?JSON.stringify(catalog.inventoryScope):'UNKNOWN — historical scope not recorded'}`));
      if(id===null)section.append(node('p','No recorded row on this side. No file identity is invented; current physical absence is not established.'));
      else {
        const f=catalog.files.find(f=>f.id===id);section.dataset.record=id;
        section.append(node('p',`Native record ID: ${f.id}`),node('p',`Original recorded relative path: ${displayName(f.path)}`),node('p',`Recorded root (text only): ${displayName(f.sourceFolder)}`),node('p',`Exact recorded bytes: ${f.bytes}`),node('p',`Recorded digest type: ${f.algorithm||'Unavailable'} · Full value: ${f.hash||'Unavailable'}`),node('p',`Recorded first-seen: ${f.firstSeen}`));
      }
      details.append(section);
    }
  };
  const draw=()=>{
    clear();if(!result)return;
    let query;try{query=exact.checked?parseExactName(filter.value):filter.value;}catch(e){status.textContent=e.message;return;}
    const visible=result.rows.filter(r=>(!kind.value||r.kind===kind.value)&&(exact.checked?r.key===query:r.key.includes(query)));
    status.textContent=`${result.totals.union} recorded relative keys in the union; reference ${result.totals.reference} rows, counterpart ${result.totals.counterpart} rows. ${Object.entries(result.counts).map(([k,n])=>classes[k]+': '+n).join('; ')}. Showing ${visible.length} keys. Counts partition recorded keys, not verified files, unique content, physical copies or backup health.`;
    for(const row of visible){const b=node('button',`${displayName(row.key)} — ${classes[row.kind]}`,{class:'result','data-key':row.key,'data-kind':row.kind,'aria-pressed':'false'});b.addEventListener('click',()=>{for(const e of results.children)e.setAttribute('aria-pressed',String(e===b));showDetail(row);});results.append(b);}
    if(!visible.length)results.append(node('p','No recorded keys in this view. Empty/all-excluded inventories are not a health or verification percentage.'));
  };
  const invalidate=()=>{epoch++;result=null;clear();status.textContent='Selection changed — accept alignment again to obtain a new comparison.';};
  reference.addEventListener('change',()=>{invalidate();run.disabled=!reference.value||Boolean(frameError);});
  for(const [control,event]of [[filter,'input'],[kind,'change'],[exact,'change']])control.addEventListener(event,()=>{epoch++;if(result)draw();else{clear();status.textContent='Filter changed while comparison is unavailable. Run comparison again.';}});
  run.addEventListener('click',async()=>{
    invalidate();const generation=epoch,ref=reference.value,other=snapshots.find(s=>s.handle!==ref)?.handle,request=crypto.randomUUID();
    status.textContent='Checking complete recorded sets from both readers…';
    try {
      const response=await fetch('/catalog-compare?'+new URLSearchParams({reference:ref,counterpart:other,request}));
      const value=await response.json();if(generation!==epoch||!panel.isConnected)return;
      if(!response.ok||!value.ok)throw Error(value.error||'Comparison unavailable');
      result=validateComparison(value,snapshots,ref,other,request);draw();
    }catch(e){if(generation!==epoch||!panel.isConnected)return;result=null;clear();status.textContent='Comparison unavailable — '+e.message;}
  });
}
