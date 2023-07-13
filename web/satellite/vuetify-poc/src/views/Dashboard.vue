// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>

        <PageTitleComponent title="Project Overview"/>
        <PageSubtitleComponent :subtitle="`Your ${limits.objectCount.toLocaleString()} files are stored in ${limits.segmentCount.toLocaleString()} segments around the world.`"/>

        <v-row class="d-flex align-center justify-center mt-2">
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Files" subtitle="Project files" :data="limits.objectCount.toLocaleString()" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Segments" subtitle="All file pieces" :data="limits.segmentCount.toLocaleString()" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Buckets" subtitle="Project buckets" :data="bucketsCount.toLocaleString()" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Access" subtitle="Project accesses" :data="accessGrantsCount.toLocaleString()" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Team" subtitle="Project members" :data="teamSize" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Billing" subtitle="Free account" data="Free" />
            </v-col>
        </v-row>

        <v-row class="d-flex align-center justify-center">
            <v-col cols="12" md="6">
                <UsageProgressComponent title="Storage" progress="40" used="10 GB Used" limit="Limit: 25GB" available="15 GB Available" cta="Need more?"/>
            </v-col>
            <v-col cols="12" md="6">
                <UsageProgressComponent title="Download" progress="60" used="15 GB Used" limit="Limit: 25GB per month" available="10 GB Available" cta="Need more?"/>
            </v-col>
            <v-col cols="12" md="6">
                <UsageProgressComponent title="Segments" progress="12" used="1,235 Used" limit="Limit: 10,000" available="8,765 Available" cta="Learn more"/>
            </v-col>
            <v-col cols="12" md="6">
                <UsageProgressComponent title="Free Tier" progress="50" used="10 GB Used" limit="Limit: $1.65" available="50% Available" cta="Upgrade to Pro"/>
            </v-col>
        </v-row>

    </v-container>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { Project, ProjectLimits } from '@/types/projects';

import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@poc/components/PageSubtitleComponent.vue';
import CardStatsComponent from '@poc/components/CardStatsComponent.vue';
import UsageProgressComponent from '@poc/components/UsageProgressComponent.vue';

const projectsStore = useProjectsStore();
const pmStore = useProjectMembersStore();
const agStore = useAccessGrantsStore();
const billingStore = useBillingStore();
const bucketsStore = useBucketsStore();

/**
 * Returns current limits from store.
 */
const limits = computed((): ProjectLimits => {
    return projectsStore.state.currentLimits;
});

/**
 * Get selected project from store.
 */
const selectedProject = computed((): Project => {
    return projectsStore.state.selectedProject;
});

/**
 * Returns current team size from store.
 */
const teamSize = computed((): number => {
    return pmStore.state.page.totalCount;
});

/**
 * Returns access grants count from store.
 */
const accessGrantsCount = computed((): number => {
    return agStore.state.page.totalCount;
});

/**
 * Returns access grants count from store.
 */
const bucketsCount = computed((): number => {
    return bucketsStore.state.page.totalCount;
});

/**
 * Lifecycle hook after initial render.
 * Fetches project limits.
 */
onMounted(async (): Promise<void> => {
    const projectID = selectedProject.value.id;
    const FIRST_PAGE = 1;

    try {
        await Promise.all([
            projectsStore.getProjectLimits(projectID),
            billingStore.getProjectUsageAndChargesCurrentRollup(),
            pmStore.getProjectMembers(FIRST_PAGE, projectID),
            agStore.getAccessGrants(FIRST_PAGE, projectID),
            bucketsStore.getBuckets(FIRST_PAGE, projectID),
        ]);
    } catch (error) { /* empty */ }
});
</script>
