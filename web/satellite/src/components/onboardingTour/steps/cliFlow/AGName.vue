// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        :is-loading="isLoading"
        title="Lets Create an Access Grant"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="permissions">
            <p class="permissions__msg">Access Grants are keys that allow access to upload, delete, and view your project’s data.</p>
            <HeaderedInput
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

import { RouteConfig } from "@/router";
import { AccessGrant} from "@/types/accessGrants";
import { ACCESS_GRANTS_ACTIONS} from "@/store/modules/accessGrants";
import { APP_STATE_MUTATIONS } from "@/store/mutationConstants";

import CLIFlowContainer from "@/components/onboardingTour/steps/common/CLIFlowContainer.vue";
import HeaderedInput from "@/components/common/HeaderedInput.vue";
import Icon from '@/../static/images/onboardingTour/accessGrant.svg';

// @vue/component
@Component({
    components: {
        CLIFlowContainer,
        HeaderedInput,
        Icon,
    }
})
export default class AGName extends Vue {
    private name = '';
    private errorMessage = '';
    private isLoading = false;

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
        this.backRoute ?
            await this.$router.push(this.backRoute).catch(() => {return; }) :
            await this.$router.push(RouteConfig.ProjectDashboard.path);
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        if (this.isLoading) return;

        if (!this.name) {
            this.errorMessage = 'Access Grant name can\'t be empty';

            return;
        }

        this.isLoading = true;

        let createdAccessGrant: AccessGrant;
        try {
            createdAccessGrant = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, this.name);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;

            return;
        }

        this.$store.commit(APP_STATE_MUTATIONS.SET_ONB_CLEAN_API_KEY, createdAccessGrant.secret);
        this.name = '';
        this.isLoading = false;

        await this.$router.push({name: RouteConfig.AGPermissions.name,});
    }

    /**
     * Returns back route from store.
     */
    private get backRoute(): string {
        return this.$store.state.appStateModule.appState.onbAGStepBackRoute;
    }
}
</script>

<style scoped lang="scss">
    .permissions {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 18px;
            line-height: 32px;
            letter-spacing: 0.15px;
            color: #4e4b66;
            margin: 10px 0 30px;
        }
    }
</style>