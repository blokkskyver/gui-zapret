let releaseItems=[],editorOriginal='',editorName='';
// A small ASCII rotor follows the last observed Windows service state.
const rotorFrames=[
 '   |   \n --O-- \n   |   ',
 ' \\   / \n   O   \n /   \\ ',
 '       \n---O---\n       ',
 ' /   \\ \n   O   \n \\   / '
];
let rotorFrame=0;
setInterval(()=>{
 if(state?.status.state!=='running'||document.hidden)return;
 rotorFrame=(rotorFrame+1)%rotorFrames.length;
 $('asciiRotor').textContent=rotorFrames[rotorFrame];
},180);
$('releaseList').onclick=()=>run(async()=>{
 releaseItems=await api().Releases();
 $('release').replaceChildren(new Option('Последний стабильный релиз',''),...releaseItems.map(r=>new Option(r.tag_name+(r.prerelease?' (предварительный)':''),r.tag_name)));
 notice('Список релизов обновлён.');
});
$('download').onclick=()=>run(async()=>{
 if(settingsDirty)throw Error('Сначала сохраните настройки.');
 if(!releaseItems.length)releaseItems=await api().Releases();
 const tag=$('release').value||releaseItems.find(r=>!r.prerelease)?.tag_name;
 if(!tag)throw Error('Стабильный релиз не найден.');
 notice('Загрузка '+tag+'…');
 await api().DownloadRelease(tag);notice('Релиз '+tag+' загружен. Примените стратегию для переключения службы.');
});
$('preview').onclick=()=>run(async()=>{
 editorName=$('strategy').value;editorOriginal=await api().ReadStrategy(editorName);
 $('source').value=editorOriginal;$('editorTitle').textContent='Стратегия: '+editorName;$('editorNotice').textContent='При сохранении создаётся резервная копия.';$('editor').showModal();
});
$('editorCancel').onclick=async()=>{if($('source').value!==editorOriginal&&!window.confirm('Закрыть редактор без сохранения?'))return;$('editor').close()};
$('editor').addEventListener('cancel',e=>{e.preventDefault();$('editorCancel').click()});
$('editorSave').onclick=async()=>{
 $('editorSave').disabled=true;
 try{const content=$('source').value;await api().SaveStrategy(editorName,editorOriginal,content);editorOriginal=content;$('editorNotice').textContent='Сохранено. Примените стратегию для обновления службы.'}catch(e){$('editorNotice').textContent=String(e)}finally{$('editorSave').disabled=false}
};
$('testSelected').onclick=()=>run(async()=>{
 if(settingsDirty)throw Error('Сначала сохраните настройки.');
 if(!await ask('Тест выбранной стратегии','Будет запущен штатный тест Flowseal только для '+$('strategy').value+'. Он временно меняет IPSet и запускает winws. Служба zapret должна быть удалена.'))return;
 await api().TestStrategy($('strategy').value);
});
