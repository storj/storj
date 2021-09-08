// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="onboarding-cli">
        <div class="onboarding-cli__container">
            <router-view />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from "@/router";
import { APP_STATE_MUTATIONS } from "@/store/mutationConstants";

// @vue/component
@Component
export default class OnbCLIStep extends Vue {
    /**
     * Lifecycle hook after initial render.
     * Sets default create bucket component's back route.
     */
    public async mounted(): Promise<void> {
        // Setting here instead of directly in store because it causes tests dependent on appStateModule to fail.
        // Probably because of mixing Vuex store and Vue-Router.
        // As a workaround we could try to store this info in Local Storage of user's browser.
        this.$store.commit(
            APP_STATE_MUTATIONS.SET_ONB_CLI_FLOW_CREATE_BUCKET_BACK_ROUTE,
            RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.GenerateAG)).path,
        )
    }
}
</script>
