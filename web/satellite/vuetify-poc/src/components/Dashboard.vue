// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-responsive class="fill-height my-3 mx-lg-3">

            <h1 class="text-h5 font-weight-bold mb-3">Project overview</h1>
            <p>Your {{ limits.objectCount }} files are stored in {{ limits.segmentCount }} segments around the world.</p>

            <div class="py-4" />

            <v-row class="d-flex align-center justify-center">
                <v-col cols="12" sm="6" md="4" lg="2">
                    <v-card title="Files" subtitle="All project files" variant="flat" :border="true" class="rounded-lg">
                        <v-card-text>
                            <v-chip rounded color="success" variant="outlined" class="font-weight-bold mb-2">{{ limits.objectCount }}</v-chip>
                        </v-card-text>
                    </v-card>
                </v-col>

                <v-col cols="12" sm="6" md="4" lg="2">
                    <v-card title="Segments" subtitle="All file pieces" variant="flat" :border="true" class="rounded-lg">
                        <v-card-text>
                            <v-chip rounded color="success" variant="outlined" class="font-weight-bold mb-2">{{ limits.segmentCount }}</v-chip>
                        </v-card-text>
                    </v-card>
                </v-col>

                <v-col cols="12" sm="6" md="4" lg="2">
                    <v-card title="Buckets" subtitle="Create bucket ➔" variant="flat" :border="true" class="rounded-lg">
                        <v-card-text>
                            <v-chip rounded color="success" variant="outlined" class="font-weight-bold mb-2">{{ bucketsCount }}</v-chip>
                        </v-card-text>
                    </v-card>
                </v-col>

                <v-col cols="12" sm="6" md="4" lg="2">
                    <v-card title="Access" subtitle="New access ➔" variant="flat" :border="true" class="rounded-lg">
                        <v-card-text>
                            <v-chip rounded color="success" variant="outlined" class="font-weight-bold mb-2">{{ accessGrantsCount }}</v-chip>
                        </v-card-text>
                    </v-card>
                </v-col>

                <v-col cols="12" sm="6" md="4" lg="2">
                    <v-card title="Team" subtitle="Invite member ➔" variant="flat" :border="true" class="rounded-lg">
                        <v-card-text>
                            <v-chip rounded color="success" variant="outlined" class="font-weight-bold mb-2">{{ teamSize }}</v-chip>
                        </v-card-text>
                    </v-card>
                </v-col>

                <v-col cols="12" sm="6" md="4" lg="2">
                    <v-card title="Billing" subtitle="Free account" variant="flat" :border="true" class="rounded-lg">
                        <v-card-text>
                            <v-chip rounded color="success" variant="outlined" class="font-weight-bold mb-2">Free</v-chip>
                        </v-card-text>
                    </v-card>
                </v-col>
            </v-row>

            <v-row class="d-flex align-center justify-center">
                <v-col cols="12" md="6">
                    <v-card title="Storage" variant="flat" :border="true" class="rounded-lg">
                        <v-card-item class="py-1">
                            <v-progress-linear color="success" model-value="40" rounded height="6">
                            </v-progress-linear>
                        </v-card-item>
                        <v-card-item>
                            <v-row>
                                <v-col>
                                    <h4>10 GB Used</h4>
                                    <p><small>Limit: 25GB</small></p>
                                </v-col>
                                <v-col>
                                    <h4 class="text-right">15 GB Available</h4>
                                    <p class="text-right"><small>Need more?</small></p>
                                </v-col>
                            </v-row>
                        </v-card-item>
                    </v-card>
                </v-col>
                <v-col cols="12" md="6">
                    <v-card title="Download" variant="flat" :border="true" class="rounded-lg">
                        <v-card-item class="py-1">
                            <v-progress-linear color="success" model-value="60" rounded height="6" />
                        </v-card-item>
                        <v-card-item>
                            <v-row>
                                <v-col>
                                    <h4>15 GB Used</h4>
                                    <p><small>Limit: 25GB per month</small></p>
                                </v-col>
                                <v-col>
                                    <h4 class="text-right">10 GB Available</h4>
                                    <p class="text-right"><small>Need more?</small></p>
                                </v-col>
                            </v-row>
                        </v-card-item>
                    </v-card>
                </v-col>
            </v-row>

            <v-row class="d-flex align-center justify-center">
                <v-col cols="12" sm="6">
                    <v-card title="Segments" variant="flat" :border="true" class="rounded-lg">
                        <v-card-item class="py-1">
                            <v-progress-linear color="success" model-value="12" rounded height="6" />
                        </v-card-item>
                        <v-card-item>
                            <v-row>
                                <v-col>
                                    <h4>1,234 Used</h4>
                                    <p><small>Limit: 10,000</small></p>
                                </v-col>
                                <v-col>
                                    <h4 class="text-right">8,765 Available</h4>
                                    <p class="text-right"><small>Need more?</small></p>
                                </v-col>
                            </v-row>
                        </v-card-item>
                    </v-card>
                </v-col>
                <v-col cols="12" sm="6">
                    <v-card title="Free Tier" variant="flat" :border="true" class="rounded-lg">
                        <v-card-item class="py-1">
                            <v-progress-linear color="success" model-value="50" rounded height="6" />
                        </v-card-item>
                        <v-card-item>
                            <v-row>
                                <v-col>
                                    <h4>50% Used</h4>
                                    <p><small>Limit: $1.65</small></p>
                                </v-col>
                                <v-col>
                                    <h4 class="text-right">50% Available</h4>
                                    <p class="text-right"><small>Add a card</small></p>
                                </v-col>
                            </v-row>
                        </v-card-item>
                    </v-card>
                </v-col>
            </v-row>
        </v-responsive>
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
