// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <div id="app">
            <router-view />
            <Notifications />
        </div>
    </v-app>
</template>

<script setup lang="ts">
import { VApp } from 'vuetify/lib';
import { onMounted } from 'vue';

import Notifications from './components/notification/Notifications.vue';

import { useVuetify } from '@/app/utils/composables';

const vuetify = useVuetify();

onMounted(() => {
    const savedTheme = localStorage.getItem('theme') || 'light';
    if (savedTheme === 'dark' && !vuetify.theme.dark) {
        vuetify.theme.dark = true;
    } else if (savedTheme === 'light' && vuetify.theme.dark) {
        vuetify.theme.dark = false;
    }
});
</script>

<style lang="scss">
@import 'static/styles/variables';

body {
    margin: 0 !important;
    position: relative;
    overflow-y: hidden;
}

.v-application {

    p,
    h1,
    h2,
    h3,
    h4 {
        margin: 0;
    }
}

#app {
    width: 100vw;
    height: 100vh;
}

@font-face {
    font-display: swap;
    font-family: 'font_regular';
    src: url('../../static/fonts/font_regular.ttf');
}

@font-face {
    font-display: swap;
    font-family: 'font_medium';
    src: url('../../static/fonts/font_medium.ttf');
}

@font-face {
    font-display: swap;
    font-family: 'font_semiBold';
    src: url('../../static/fonts/font_semiBold.ttf');
}

@font-face {
    font-display: swap;
    font-family: 'font_bold';
    src: url('../../static/fonts/font_bold.ttf');
}
</style>
