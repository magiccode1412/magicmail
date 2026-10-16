// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { setRouter } from './api/request'
import './styles/variables.css'
import './styles/themes.css'
import './styles/global.css'
import './styles/responsive.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)
setRouter(router)

app.mount('#app')
