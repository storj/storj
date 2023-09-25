// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * main.js
 *
 * Bootstraps Vuetify and other plugins then mounts the App`
 */

// Components
import App from './App.vue'

// Composables
import { createApp } from 'vue'

// Styles
import './styles/settings.scss'
// import './styles/styles.scss'

// Plugins
import { registerPlugins } from '@/plugins'

const app = createApp(App)

registerPlugins(app)

app.mount('#app')
