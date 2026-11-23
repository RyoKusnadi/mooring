import { mount } from 'svelte'
import App from './App.svelte'
import './tokens.css'
import './app.css'
import './base.css'
import './forms.css'
import './controls.css'
import './menu.css'
import './dialog.css'

const target = document.getElementById('app')
if (!target) throw new Error('index.html is missing #app')
mount(App, { target })
