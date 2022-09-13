// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <ProgressBar />
        <router-view />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProgressBar from '@/components/onboardingTour/steps/cliFlow/ProgressBar.vue';

// @vue/component
@Component({
    components: {
        ProgressBar,
    },
})
export default class OnbCLIStep extends Vue {}
</script>
