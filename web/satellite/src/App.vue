// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="app">
        <router-view/>
        <!-- Area for displaying notification -->
        <NotificationArea/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import NotificationArea from '@/components/notifications/NotificationArea.vue';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        NotificationArea
    },
})

export default class App extends Vue {
}
</script>

<style lang="scss">

    @font-face {
        font-family: "font_regular";
        src: url("../static/fonts/font_regular.ttf");
    }

    @font-face {
        font-family: "font_medium";
        src: url("../static/fonts/font_medium.ttf");
    }

    @font-face {
        font-family: "font_bold";
        src: url("../static/fonts/font_bold.ttf");
    }

    a {
        cursor: pointer;
    }

    input,
    textarea {
        font-family: inherit;
        font-weight: 600;
        border: 1px solid rgba(56, 75, 101, 0.4);
        color: #354049;
        caret-color: #2683FF;
    }
</style>
