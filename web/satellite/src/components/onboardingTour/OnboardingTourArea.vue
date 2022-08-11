// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="tour-area">
        <router-view />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class OnboardingTourArea extends Vue {}
</script>

<style scoped lang="scss">
.tour-area {
    padding: 30px 0;
    width: 100%;
    height: calc(100% - 60px);
    display: flex;
    flex-direction: column;
    align-items: center;
}

@media screen and (max-width: 760px) {

    .tour-area {
        width: 88% !important;
    }
}
</style>
