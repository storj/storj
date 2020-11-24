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

    /**
     * Lifecycle hook after initial render.
     * Sets local key from props value.
     */
    public mounted(): void {
        if (!this.$route.params.key) {
            this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);
        }

        this.key = this.$route.params.key;
    }

    /**
     * Holds on back button click logic.
     * Redirects to previous step.
     */
    public onBackClick(): void {
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
        this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.UploadStep)).path);
    }

    /**
     * Holds on copy button click logic.
     * Copies token to clipboard.
     */
    public onCopyClick(): void {
        this.$copyText(this.key);
        this.$notify.success('Token was copied successfully');
    }
}
</script>

<style scoped lang="scss" src="./CLIStep.scss"></style>