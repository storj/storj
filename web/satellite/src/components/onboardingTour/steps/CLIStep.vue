// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <ProgressBar />
        <router-view />
    </div>
</template>

<script setup lang="ts">
import ProgressBar from '@/components/onboardingTour/steps/cliFlow/ProgressBar.vue';
</script>
