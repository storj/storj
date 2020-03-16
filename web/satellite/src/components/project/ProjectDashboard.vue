// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-overview">
        <TabNavigation
            v-if="isProjectSelected"
            class="project-overview__navigation"
            :navigation="navigation"
        />
        <router-view v-if="isProjectSelected"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import TabNavigation from '@/components/navigation/TabNavigation.vue';

import { NavigationLink } from '@/types/navigation';

@Component({
    components: {
        TabNavigation,
    },
})
export default class ProjectDashboard extends Vue {
    // TODO: make type for project routes
    public navigation: NavigationLink[] = [
        new NavigationLink('/project-dashboard/details', 'Details'),
        new NavigationLink('/project-dashboard/usage-report', 'Report'),
    ];

    /**
     * Indicates if project is selected.
     */
    public get isProjectSelected(): boolean {
        return this.$store.getters.selectedProject.id !== '';
    }
}
</script>

<style scoped lang="scss">
    .project-overview {
        padding: 40px 65px 55px 65px;
        position: relative;

        &__navigation {
            position: absolute;
            right: 65px;
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
