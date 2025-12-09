// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VApp>
        <div id="app">
            <router-view />
            <Notifications />
        </div>
    </VApp>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import { VApp } from 'vuetify/components';
import { useTheme } from 'vuetify';

import Notifications from './components/notification/Notifications.vue';

const theme = useTheme();

onMounted(() => {
    const savedTheme = localStorage.getItem('theme') || 'light';
    if (savedTheme === 'dark' && !theme.global.current.value.dark) {
        theme.change('dark');
    } else if (savedTheme === 'light' && theme.global.current.value.dark) {
        theme.change('light');
    }
});
</script>

<style lang="scss">
@import '../../static/styles/variables';

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
