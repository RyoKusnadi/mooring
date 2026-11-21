import { mount } from 'svelte'
import App from './App.svelte'
import './app.css'
import './controls.css'
import './dialog.css'

const target = document.getElementById('app')
if (!target) throw new Error('index.html is missing #app')
mount(App, { target })
