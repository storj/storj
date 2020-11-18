// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>

</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';

@Component
export default class ResultStep extends Vue {
    public access: string = '';

    public mounted(): void {
        if (!this.$route.params.access) {
            this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);
        }

        this.access = this.$route.params.access;
    }
}
</script>
