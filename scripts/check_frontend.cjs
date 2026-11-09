// Boot the real scripts against IDs from the real HTML to catch missing controls.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const root = path.resolve(__dirname, '..', 'frontend');
const html = fs.readFileSync(path.join(root, 'index.html'), 'utf8');
const elements = new Map([...html.matchAll(/id="([^"]+)"/g)].map(([, id]) => [id, {
  dataset: {}, value: '', title: '', textContent: '', options: [],
  classList: { toggle() {} }, setAttribute() {}, addEventListener() {},
  showModal(){this.open=true},close(){this.open=false},replaceChildren(...items){this.options=items},
}]));
const timers = [];
let snapshots = 0;
let completeSnapshot;
const context = vm.createContext({
  document: { getElementById: id => elements.get(id) || null,
    documentElement: { dataset: {} }, querySelectorAll: () => [], querySelector:()=>null, hidden: false },
  window: { addEventListener() {}, runtime: {}, go: { main: { App: {
    Snapshot() { snapshots++; return new Promise(resolve => {completeSnapshot=resolve}); },
  } } } },
  localStorage: { getItem() { return null; } },
  setInterval(fn) { timers.push(fn); }, console,
  Option: function(text,value){this.text=text;this.value=value},
});
for (const name of ['theme.js', 'app.js', 'features.js', 'lists.js']) {
  vm.runInContext(fs.readFileSync(path.join(root, name), 'utf8'), context, { filename: name });
}
for (const id of ['browse','admin','strategy','preview','install','start','stop','remove',
  'save','menu','lists','logs','close','minimize','repository','tests','diagnostics',
  'releaseList','download','testSelected','editorSave','listsTab','controlTab','listSave']) {
  const event = id === 'strategy' ? 'onchange' : 'onclick';
  assert.equal(typeof elements.get(id)[event], 'function', `${id}: missing ${event}`);
}
// A delayed backend response must not start a queue of concurrent status requests.
for (let i=0; i<10; i++) for (const timer of timers) timer();
assert.equal(snapshots, 1, 'status requests overlapped');
console.log('PASS: frontend startup, handlers and non-overlapping status polling');
completeSnapshot({root:'Documents/zapret',rootValid:false,admin:false,strategies:[],status:{state:'not-installed'},game:{mode:'disabled',tcp:'1024-65535',udp:'1024-65535'},logs:[]});
setImmediate(()=>{
 assert.equal(elements.get('installPrompt').open,true,'missing distribution must offer installation');
 assert.equal(elements.get('install').disabled,true,'cannot install a service without a distribution');
 assert.equal(elements.get('start').disabled,true,'cannot control services without elevation');
 console.log('PASS: portable first-run installation prompt and privilege controls');
});
