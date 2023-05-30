// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="wrap">
        <ProgressBar />
        <router-view />
    </div>
</template>

<script setup lang="ts">
import ProgressBar from '@/components/onboardingTour/steps/cliFlow/ProgressBar.vue';
</script>

<style scoped lang="scss">
.wrap {
    @media screen and (width <= 600px) {
        width: 100%;
    }
}
</style>
