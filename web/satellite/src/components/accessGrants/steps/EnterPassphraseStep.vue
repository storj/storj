// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="enter-passphrase">
        <h1 class="enter-passphrase__title">Encryption Passphrase</h1>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';

@Component
export default class EnterPassphraseStep extends Vue {
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
}
</script>
