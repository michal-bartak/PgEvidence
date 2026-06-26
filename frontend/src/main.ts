import './style.css'
import App from './App.svelte'

// WebKit (WKWebView) auto-capitalizes the first letter of text inputs by default,
// which is wrong for SQL, hostnames, paths, etc. Disable it (plus autocorrect) on
// every field as it gains focus — covers dynamically created inputs too.
document.addEventListener('focusin', (e) => {
  const el = e.target as HTMLElement
  if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement) {
    el.setAttribute('autocapitalize', 'off')
    el.setAttribute('autocorrect', 'off')
    el.setAttribute('spellcheck', 'false')
  }
})

const app = new App({
  target: document.getElementById('app')
})

export default app
