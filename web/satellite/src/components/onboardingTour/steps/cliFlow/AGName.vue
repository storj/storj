// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        :is-loading="isLoading"
        title="Create an Access Grant"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="permissions">
            <p class="permissions__msg">Access Grants are keys that allow access to upload, delete, and view your project’s data.</p>
            <VInput
                label="Access Grant Name"
                placeholder="Enter a name here..."
                :error="errorMessage"
                aria-roledescription="name"
                @setData="onChangeName"
            />
        </template>
    </CLIFlowContainer>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { AccessGrant } from '@/types/accessGrants';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import CLIFlowContainer from '@/components/onboardingTour/steps/common/CLIFlowContainer.vue';
import VInput from '@/components/common/VInput.vue';

import Icon from '@/../static/images/onboardingTour/accessGrant.svg';

// @vue/component
@Component({
    components: {
        CLIFlowContainer,
        VInput,
        Icon,
    },
})
export default class AGName extends Vue {
    private name = '';
    private errorMessage = '';
    private isLoading = false;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Changes name data from input value.
     * @param value
     */
    public onChangeName(value: string): void {
        this.name = value.trim();
        this.errorMessage = '';
    }

    /**
     * Holds on back button click logic.
     * Navigates to previous screen.
     */
    public async onBackClick(): Promise<void> {
        this.analytics.pageVisit(RouteConfig.OverviewStep.path);
        this.backRoute ?
            await this.$router.push(this.backRoute).catch(() => {return; }) :
            await this.$router.push({ name: RouteConfig.OverviewStep.name });
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        if (this.isLoading) return;

        if (!this.name) {
            this.errorMessage = 'Access Grant name can\'t be empty';
            this.analytics.errorEventTriggered(AnalyticsErrorEventSource.ONBOARDING_NAME_STEP);

            return;
        }

        this.isLoading = true;

        let createdAccessGrant: AccessGrant;
        try {
            createdAccessGrant = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, this.name);

            await this.$notify.success('New clean access grant was generated successfully.');
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.ONBOARDING_NAME_STEP);
            return;
        } finally {
            this.isLoading = false;
        }

        this.$store.commit(APP_STATE_MUTATIONS.SET_ONB_CLEAN_API_KEY, createdAccessGrant.secret);
        this.name = '';

        this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.AGPermissions)).path);
        await this.$router.push({ name: RouteConfig.AGPermissions.name });
    }

    /**
     * Returns back route from store.
     */
    private get backRoute(): string {
        return this.$store.state.appStateModule.viewsState.onbAGStepBackRoute;
    }
}
</script>

<style scoped lang="scss">
    .permissions {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 16px;
            line-height: 24px;
            color: #4e4b66;
            margin-bottom: 20px;
        }
    }
</style>
