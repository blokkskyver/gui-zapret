(() => {
 const root=document.documentElement;
 let theme='light';
 try{if(localStorage.getItem('zapret-theme')==='dark')theme='dark'}catch{}
 root.dataset.theme=theme;
 window.addEventListener('DOMContentLoaded',()=>{
  const button=document.getElementById('theme');
  function render(){
   const dark=root.dataset.theme==='dark';
   button.textContent=dark?'☀':'☾';
   button.title=dark?'Включить светлую тему':'Включить тёмную тему';
   button.setAttribute('aria-label',button.title);
   button.setAttribute('aria-pressed',String(dark));
  }
  button.onclick=()=>{
   root.dataset.theme=root.dataset.theme==='dark'?'light':'dark';
   try{localStorage.setItem('zapret-theme',root.dataset.theme)}catch{}
   render();
  };
  render();
 });
})();
