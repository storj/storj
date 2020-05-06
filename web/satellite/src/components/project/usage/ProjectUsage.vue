// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-usage">
        <div class="project-usage__title-area">
            <h1 class="project-usage__title-area__title">Project Usage</h1>
            <a
                class="project-usage__title-area__link"
                href="https://support.tardigrade.io/hc/en-us/requests/new?ticket_form_id=360000683212"
                target="_blank"
            >
                Request Limit Increase ->
            </a>
        </div>
        <div class="project-usage__usage-area">
            <UsageArea
                class="project-usage__usage-area__storage-used"
                title="Storage"
                :used="storageUsed"
                :limit="storageLimit"
            />
            <UsageArea
                class="project-usage__usage-area__bandwidth-used"
                title="Bandwidth"
                :used="bandwidthUsed"
                :limit="bandwidthLimit"
            />
        </div>
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
        padding: 30px;
        width: calc(70% - 60px);
        font-family: 'font_regular', sans-serif;
        background-color: #fff;
        border-radius: 6px;

        &__title-area {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 25px;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 18px;
                line-height: 18px;
                color: #354049;
                margin: 0;
            }

            &__link {
                font-size: 14px;
                line-height: 14px;
                color: #2683ff;
            }
        }

        &__usage-area {
            width: 100%;
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__storage-used,
            &__bandwidth-used {
                width: calc(50% - 10px);
            }

            &__storage-used {
                margin-right: 20px;
            }
        }
    }
</style>
