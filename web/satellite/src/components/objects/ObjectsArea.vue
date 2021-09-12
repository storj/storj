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
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { MetaUtils } from '@/utils/meta';

@Component
export default class ObjectsArea extends Vue {
    /**
     * Lifecycle hook after initial render.
     * Redirects if flow is disabled.
     */
    public async mounted(): Promise<void> {
        if (await JSON.parse(MetaUtils.getMetaContent('file-browser-flow-disabled'))) {
            await this.$router.push(RouteConfig.ProjectDashboard.path);
        }
    }

    /**
     * Lifecycle hook before component destroying.
     * Clears objects VUEX state.
     */
    public beforeDestroy() {
        this.$store.dispatch(OBJECTS_ACTIONS.CLEAR);
    }
}
</script>

<style scoped lang="scss">
    .objects-area {
        padding: 20px 45px;
    }
</style>
