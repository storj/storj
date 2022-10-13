// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="name-step" :class="{ 'border-radius': isOnboardingTour }">
        <h1 class="name-step__title" aria-roledescription="name-ag-title">Name Your Access Grant</h1>
        <p class="name-step__sub-title">Enter a name for your new Access grant to get started.</p>
        <VInput
            label="Access Grant Name"
            placeholder="Enter a name here..."
            :error="errorMessage"
            @setData="onChangeName"
        />
        <div class="name-step__buttons-area">
            <VButton
                v-if="!isOnboardingTour"
                class="cancel-button"
                label="Cancel"
                width="50%"
                height="48px"
                :on-press="onCancelClick"
                is-white="true"
                :is-disabled="isLoading"
            />
            <VButton
                label="Next"
                width="50%"
                height="48px"
                :on-press="onNextClick"
                :is-disabled="isLoading"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { AccessGrant } from '@/types/accessGrants';
import { AnalyticsHttpApi } from '@/api/analytics';

import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';

// @vue/component
@Component({
    components: {
        VInput,
        VButton,
    },
})
export default class NameStep extends Vue {
    private name = '';
    private errorMessage = '';
    private isLoading = false;
    private key = '';

    private readonly FIRST_PAGE = 1;

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
     * Holds on cancel button click logic.
     */
    public onCancelClick(): void {
        this.onChangeName('');
        this.analytics.pageVisit(RouteConfig.AccessGrants.path);
        this.$router.push(RouteConfig.AccessGrants.path);
    }

    /**
     * Holds on next button click logic.
     * Creates AccessGrant common entity.
     */
    public async onNextClick(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        if (!this.name) {
            this.errorMessage = 'Access Grant name can\'t be empty';

            return;
        }

        this.isLoading = true;

        // Check if at least one project exists.
        // Used like backwards compatibility for the old accounts without any project.
        if (this.$store.getters.projects.length === 0) {
            try {
                await this.$store.dispatch(PROJECTS_ACTIONS.CREATE_DEFAULT_PROJECT);
            } catch (error) {
                this.isLoading = false;

                return;
            }
        }

        let createdAccessGrant: AccessGrant;
        try {
            createdAccessGrant = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, this.name);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;

            return;
        }

        this.key = createdAccessGrant.secret;
        this.name = '';

        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch Access Grants. ${error.message}`);

            this.isLoading = false;
        }

        this.isLoading = false;

        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.PermissionsStep)).path);
        await this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.PermissionsStep)).name,
            params: {
                key: this.key,
            },
        });
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
    }
}
</script>

<style scoped lang="scss">
    .name-step {
        height: calc(100% - 60px);
        padding: 30px 65px;
        font-family: 'font_regular', sans-serif;
        font-style: normal;
        display: flex;
        flex-direction: column;
        align-items: center;
        background-color: #fff;
        border-radius: 0 6px 6px 0;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-weight: bold;
            font-size: 22px;
            line-height: 27px;
            color: #000;
            margin: 0 0 10px;
        }

        &__sub-title {
            font-weight: normal;
            font-size: 16px;
            line-height: 21px;
            color: #000;
            text-align: center;
            margin: 0 0 80px;
        }

        &__buttons-area {
            width: 100%;
            display: flex;
            align-items: center;
            justify-content: center;
            margin-top: 130px;
        }
    }

    .cancel-button {
        margin-right: 15px;
    }

    .border-radius {
        border-radius: 6px;
    }
</style>
