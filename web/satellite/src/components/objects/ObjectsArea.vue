// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="objects-area">
        <router-view/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { LocalData, UserIDPassSalt } from '@/utils/localData';

@Component
export default class ObjectsArea extends Vue {
    /**
     * Lifecycle hook after initial render.
     * Chooses correct route.
     */
    public async mounted(): Promise<void> {
        const DUPLICATE_NAV_ERROR: string = 'NavigationDuplicated';
        const idPassSalt: UserIDPassSalt | null = LocalData.getUserIDPassSalt();
        if (idPassSalt && idPassSalt.userId === this.$store.getters.user.id) {
            try {
                await this.$router.push(RouteConfig.Objects.with(RouteConfig.EnterPassphrase).path);

                return;
            } catch (error) {
                if (error.name === DUPLICATE_NAV_ERROR) {
                    return;
                }

                await this.$notify.error(error.message);
            }
        }

        try {
            await this.$router.push(RouteConfig.Objects.with(RouteConfig.CreatePassphrase).path);
        } catch (error) {
            if (error.name === DUPLICATE_NAV_ERROR) {
                return;
            }

            await this.$notify.error(error.message);
        }
    }
}
</script>

<style scoped lang="scss">
    .objects-area {
        padding: 20px 45px;
    }
</style>
