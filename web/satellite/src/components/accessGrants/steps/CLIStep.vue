// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./CLIStep.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';

import { RouteConfig } from '@/router';

@Component({
    components: {
        BackIcon,
        VButton,
    },
})
export default class CLIStep extends Vue {
    public key: string = '';
    public restrictedKey: string = '';

    /**
     * Lifecycle hook after initial render.
     * Sets local key from props value.
     */
    public mounted(): void {
        if (!this.$route.params.key && !this.$route.params.restrictedKey) {
            this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);

            return;
        }

        this.key = this.$route.params.key;
        this.restrictedKey = this.$route.params.restrictedKey;
    }

    /**
     * Holds on back button click logic.
     * Redirects to previous step.
     */
    public onBackClick(): void {
        if (this.isOnboardingTour) {
            this.$router.push({
                name: RouteConfig.OnboardingTour.with(RouteConfig.AccessGrant.with(RouteConfig.AccessGrantPermissions)).name,
                params: {
                    key: this.key,
                },
            });

            return;
        }

        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.PermissionsStep)).name,
            params: {
                key: this.key,
            },
        });
    }

    /**
     * Holds on done button click logic.
     * Redirects to upload step.
     */
    public onDoneClick(): void {
        if (this.isOnboardingTour) {
            this.$router.push(RouteConfig.ProjectDashboard.path);

            return;
        }

        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.UploadStep)).name,
            params: {
                isUplinkSectionEnabled: 'true',
            },
        });
    }

    /**
     * Holds on copy button click logic.
     * Copies token to clipboard.
     */
    public onCopyClick(): void {
        this.$copyText(this.restrictedKey);
        this.$notify.success('Token was copied successfully');
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
    }
}
</script>

<style scoped lang="scss" src="./CLIStep.scss"></style>