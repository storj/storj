// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-overview">
        <TabNavigation
            v-if="isProjectSelected"
            class="project-overview__navigation"
            :navigation="navigation"/>
        <router-view v-if="isProjectSelected"/>
        <EmptyState
            v-if="!isProjectSelected"
            mainTitle="Create your first project"
            additional-text='<p>Please click the button <b>"New Project"</b> in the right corner</p>'
            :imageSource="emptyImage" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import EmptyState from '@/components/common/EmptyStateArea.vue';
import TabNavigation from '@/components/navigation/TabNavigation.vue';

import { NavigationLink } from '@/types/navigation';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

@Component({
    components: {
        EmptyState,
        TabNavigation,
    },
})
export default class ProjectOverviewArea extends Vue {
    // TODO: make type for project routes
    public navigation: NavigationLink[] = [
        new NavigationLink('/project-overview/details', 'Details'),
        new NavigationLink('/project-overview/usage-report', 'Report'),
    ];

    public get isProjectSelected(): boolean {
        return this.$store.getters.selectedProject.id !== '';
    }

    public get emptyImage(): string {
        return EMPTY_STATE_IMAGES.PROJECT;
    }
}
</script>

<style scoped lang="scss">
    .project-overview {
        padding: 44px 55px 55px 55px;
        position: relative;
        height: 85vh;

        &__navigation {
            position: absolute;
            right: 55px;
            top: 44px;
            z-index: 99;
        }
    }

    @media screen and (max-width: 1024px) {
        .project-overview {
            padding: 44px 40px 55px 40px;

            &__navigation {
                right: 40px;
            }
        }
    }
</style>
