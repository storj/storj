// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <router-view />
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { RouteConfig } from "@/router";

// @vue/component
@Component
export default class OnbCLIStep extends Vue {
    /**
     * Mounted hook before initial render.
     * Checks CLI flow status and redirects if needed.
     */
    public beforeMount(): void {
        if (!this.$store.state.appStateModule.isNewOnbCLIFlow) {
            this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OldOverviewStep).path)
        }
    }
}
</script>
