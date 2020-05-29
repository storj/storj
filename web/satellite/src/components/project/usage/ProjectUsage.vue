// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-usage">
        <UsageArea
            class="project-usage__storage-used"
            title="Storage"
            :used="storageUsed"
            :limit="storageLimit"
        />
        <UsageArea
            class="project-usage__bandwidth-used"
            title="Bandwidth"
            :used="bandwidthUsed"
            :limit="bandwidthLimit"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import UsageArea from '@/components/project/usage/UsageArea.vue';

import { PROJECTS_ACTIONS } from '@/store/modules/projects';

@Component({
    components: {
        UsageArea,
    },
})
export default class ProjectUsage extends Vue {
    /**
     * Lifecycle hook after initial render.
     * Fetches project limits.
     */
    public async mounted(): Promise<void> {
        if (!this.$store.getters.selectedProject.id) {
            return;
        }

        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Returns bandwidth used amount from store.
     */
    public get bandwidthUsed(): number {
        return this.$store.state.projectsModule.currentLimits.bandwidthUsed;
    }

    /**
     * Returns bandwidth limit amount from store.
     */
    public get bandwidthLimit(): number {
        return this.$store.state.projectsModule.currentLimits.bandwidthLimit;
    }

    /**
     * Returns storage used amount from store.
     */
    public get storageUsed(): number {
        return this.$store.state.projectsModule.currentLimits.storageUsed;
    }

    /**
     * Returns storage limit amount from store.
     */
    public get storageLimit(): number {
        return this.$store.state.projectsModule.currentLimits.storageLimit;
    }
}
</script>

<style scoped lang="scss">
    .project-usage {
        width: 100%;
        font-family: 'font_regular', sans-serif;
        border-radius: 6px;
        display: flex;
        align-items: center;
        justify-content: space-between;

        &__storage-used,
        &__bandwidth-used {
            width: calc(50% - 20px);
        }

        &__storage-used {
            margin-right: 40px;
        }
    }
</style>
