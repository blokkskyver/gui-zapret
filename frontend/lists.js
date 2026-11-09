let listCatalog=[],listDoc=null,listName='',listBaseline='';
const listDirty=()=>listDoc&&$('listSource').value!==listBaseline;
async function discardListChanges(){return !listDirty()||await ask('Несохранённый список','Отменить изменения в '+listName+'?')}
function setListTab(active){
 $('controlPage').hidden=active;$('listsPage').hidden=!active;
 $('controlTab').setAttribute('aria-selected',String(!active));$('listsTab').setAttribute('aria-selected',String(active));
}
async function readList(name){
 const doc=await api().ReadList(name);
 listDoc=doc;listName=name;$('listSelect').value=name;$('listSource').value=doc.content;listBaseline=$('listSource').value;
 $('listDescription').textContent=listCatalog.find(l=>l.name===name)?.description||'';
 $('listState').textContent='Резервная копия при сохранении';$('listSource').scrollTop=0;
}
async function openLists(){
 if(busy)return;
 if(listDoc&&listDoc.root===state?.root){setListTab(true);return}
 if(!await discardListChanges())return;
 await run(async()=>{
  listCatalog=await api().Lists();listDoc=null;listName='';$('listSource').value='';
  $('listSelect').replaceChildren(...listCatalog.map(l=>new Option(l.name,l.name)));
  if(listCatalog.length){const first=listCatalog.find(l=>l.name==='list-general-user.txt')||listCatalog[0];await readList(first.name)}
  else{$('listDescription').textContent='В папке lists нет текстовых списков.';$('listState').textContent='Нет списков'}
  setListTab(true);
 });
}
$('listsTab').onclick=openLists;$('lists').onclick=openLists;
$('controlTab').onclick=()=>setListTab(false);
$('listSelect').onchange=async()=>{
 const next=$('listSelect').value;$('listSelect').value=listName;
 if(!await discardListChanges())return;
 await run(()=>readList(next));
};
$('listReload').onclick=async()=>{
 if(!listName||!await discardListChanges())return;
 await run(()=>readList(listName));
};
$('listSource').oninput=()=>{$('listState').textContent=listDirty()?'Есть несохранённые изменения':'Резервная копия при сохранении'};
$('listSave').onclick=()=>run(async()=>{
 if(!listDoc)throw Error('Сначала выберите список.');
 const content=$('listSource').value;
 await api().SaveList(listDoc.root,listName,listDoc.content,content);
 listDoc.content=content;listBaseline=content;
 $('listState').textContent='Сохранено';notice('Список сохранён. Перезапустите службу для применения.');
});
// Arrow keys switch the two tabs without discarding the editor buffer.
for(const id of ['controlTab','listsTab'])$(id).addEventListener('keydown',e=>{
 if(!['ArrowLeft','ArrowRight','Home','End'].includes(e.key))return;
 e.preventDefault();const target=e.key==='Home'?'controlTab':e.key==='End'?'listsTab':id==='controlTab'?'listsTab':'controlTab';$(target).focus();$(target).click();
});
