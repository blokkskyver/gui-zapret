const $=id=>document.getElementById(id);
const api=()=>window.go.main.App;
let busy=false, state=null, settingsDirty=false;
function notice(text,error=false){$('notice').textContent=text;$('notice').classList.toggle('error',error)}
function lock(value){busy=value;document.querySelectorAll('button:not(dialog button):not(#close):not(#minimize):not(#theme)').forEach(b=>b.disabled=value);$('activity').textContent=value?'Выполняется…':'Готово';if(!value && state)permissions()}
function permissions(){
 const admin=state?.admin===true;
 for(const id of ['install','start','stop','remove','tests','diagnostics','menu','testSelected']){
  const button=$(id);
  if(button.dataset.originalTitle===undefined)button.dataset.originalTitle=button.title;
  button.disabled=busy||!admin||(['install','testSelected'].includes(id)&&!$('strategy').value);
  button.title=admin?button.dataset.originalTitle:'Сначала нажмите «Повысить права»';
 }
 $('admin').hidden=admin;
}
permissions();
async function run(fn){if(busy)return;lock(true);notice('');try{await fn()}catch(e){notice(String(e),true)}finally{lock(false);await refresh()}}
async function refresh(){if(busy)return;try{const s=await api().Snapshot();state=s;$('root').textContent=s.root;$('root').title=s.root;
const labels={'running':'Служба работает','stopped':'Остановлена','not-installed':'Не установлена','starting':'Запускается…','stopping':'Останавливается…'};
$('status').textContent=labels[s.status.state]||s.status.state;$('led').classList.toggle('running',s.status.state==='running');$('installed').textContent=s.status.strategy||'Служба Windows';$('installed').title=s.status.command||'';
const previous=$('strategy').value;const names=s.strategies||[];const installed=names.find(n=>n.replace(/\.bat$/i,'')===(s.status.strategy||'').replace(/\.bat$/i,''));if(JSON.stringify(names)!==JSON.stringify(Array.from($('strategy').options,o=>o.value))){$('strategy').replaceChildren(...names.map(n=>new Option(n.replace(/\.bat$/i,''),n)));$('strategy').value=names.includes(previous)?previous:installed|| (names.includes('general.bat')?'general.bat':names[0]||'')}
if(!settingsDirty){$('mode').value=s.game.mode;$('tcp').value=s.game.tcp;$('udp').value=s.game.udp;$('updates').checked=s.autoUpdates}
const log=(s.logs||[]).join('\n');if($('log').textContent!==log){$('log').textContent=log;$('log').scrollTop=$('log').scrollHeight}permissions();if(s.error)notice(s.error,true);
}catch(e){notice('Не удалось прочитать состояние: '+e,true)}}
function ask(title,text,confirm=true){return new Promise(resolve=>{$('dialogTitle').textContent=title;$('dialogText').textContent=text;$('confirm').hidden=!confirm;$('cancel').textContent=confirm?'Отмена':'Закрыть';let accepted=false;$('confirm').onclick=()=>{accepted=true;$('dialog').close()};$('cancel').onclick=()=>$('dialog').close();$('dialog').onclose=()=>resolve(accepted);$('dialog').showModal()})}
$('browse').onclick=()=>run(()=>api().ChooseRoot());$('admin').onclick=()=>run(()=>api().Elevate());$('refresh').onclick=refresh;
$('strategy').onchange=()=>state&&permissions();
$('preview').onclick=()=>run(async()=>{const args=await api().Preview($('strategy').value);await ask('Параметры winws.exe',args.join('\n'),false)});
for(const action of ['install','start','stop','remove'])$(action).onclick=()=>run(async()=>{
if(action==='install'&&settingsDirty)throw new Error('Сначала сохраните настройки Game Filter.');
if(action==='remove'&&!await ask('Удалить службу zapret?','Служба будет остановлена и удалена из автозапуска. Файлы Flowseal останутся на диске.'))return;
await api().Action(action,$('strategy').value);notice('Операция выполнена.');
});
for(const id of ['mode','tcp','udp','updates'])$(id).addEventListener('input',()=>settingsDirty=true);
$('save').onclick=()=>run(async()=>{await api().SaveSettings({mode:$('mode').value,tcp:$('tcp').value.trim(),udp:$('udp').value.trim()},$('updates').checked);settingsDirty=false;notice('Сохранено. Примените стратегию, чтобы обновить службу.')});
for(const tool of ['menu','lists','logs'])$(tool).onclick=()=>run(()=>api().OpenTool(tool));
refresh();setInterval(()=>{if(!busy&&!document.hidden)refresh()},4000);
$('close').onclick=()=>window.runtime.Quit();
$('minimize').onclick=()=>window.runtime.WindowMinimise();
$('repository').onclick=()=>api().OpenRepository();
for(const tool of ['tests','diagnostics'])$(tool).onclick=()=>run(async()=>{
if(tool==='diagnostics'&&!await ask('Запустить диагностику Flowseal?','Штатная диагностика может включить TCP timestamps и удалить конфликтующие службы WinDivert. Запросы очистки Discord появятся в отдельном окне.'))return;
await api().OpenTool(tool);
});
