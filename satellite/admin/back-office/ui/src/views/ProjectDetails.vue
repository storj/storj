// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row>
            <v-col cols="12" md="8">
                <PageTitleComponent title="Project Details" />

                <v-chip variant="outlined" color="default" class="mr-2 mb-2 mb-md-0 pr-4" router-link to="/account-details">
                    <v-tooltip activator="parent" location="top">
                        Go to account
                    </v-tooltip>
                    <template #prepend>
                        <svg class="mr-1" width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M12.1271 6C14.1058 6 15.7099 7.60408 15.7099 9.58281C15.7099 10.7701 15.1324 11.8226 14.2429 12.4745C16.0273 13.1299 17.4328 14.5717 18.0402 16.3804C18.059 16.4363 18.077 16.4925 18.0942 16.5491L18.1195 16.6342C18.2377 17.0429 18.0022 17.4701 17.5934 17.5883C17.5239 17.6084 17.4518 17.6186 17.3794 17.6186H6.764C6.34206 17.6186 6 17.2765 6 16.8545C6 16.7951 6.00695 16.7358 6.02069 16.678L6.02974 16.6434C6.05458 16.5571 6.08121 16.4714 6.10959 16.3866C6.7237 14.5517 8.15871 13.0936 9.97792 12.4495C9.10744 11.7961 8.54432 10.7552 8.54432 9.58281C8.54432 7.60408 10.1484 6 12.1271 6ZM12.076 13.2168C9.95096 13.2168 8.07382 14.5138 7.29168 16.4355L7.26883 16.4925H16.8831L16.8826 16.4916C16.1224 14.5593 14.2607 13.2444 12.1429 13.2173L12.076 13.2168ZM12.1271 7.12603C10.7703 7.12603 9.67035 8.22596 9.67035 9.58281C9.67035 10.9397 10.7703 12.0396 12.1271 12.0396C13.4839 12.0396 14.5839 10.9397 14.5839 9.58281C14.5839 8.22596 13.4839 7.12603 12.1271 7.12603Z"
                                fill="currentColor"
                            />
                        </svg>
                    </template>
                    itacker@gmail.com
                </v-chip>

                <img src="@/assets/icon-right.svg" alt="Project" width="10" class="mr-2">

                <v-chip class="mr-2 mr-2 mb-2 mb-md-0 pr-4 font-weight-medium" variant="tonal" color="default">
                    <template #prepend>
                        <svg class="mr-1" width="24" height="24" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M16.2231 7.08668L16.2547 7.10399L23.4149 11.2391C23.6543 11.3774 23.7829 11.6116 23.8006 11.8529L23.8021 11.8809L23.8027 11.9121V20.1078C23.8027 20.3739 23.6664 20.6205 23.4432 20.7624L23.4136 20.7803L16.2533 24.8968C16.0234 25.029 15.7426 25.0342 15.5088 24.9125L15.4772 24.8951L8.38642 20.7787C8.15725 20.6457 8.01254 20.4054 8.00088 20.1422L8 20.1078L8.00026 11.8975L8 11.8738C8.00141 11.6177 8.12975 11.3687 8.35943 11.2228L8.38748 11.2058L15.4783 7.10425C15.697 6.97771 15.9622 6.96636 16.1893 7.07023L16.2231 7.08668ZM22.251 13.2549L16.6424 16.4939V22.8832L22.251 19.6588V13.2549ZM9.55175 13.2614V19.6611L15.0908 22.8766V16.4916L9.55175 13.2614ZM15.8669 8.67182L10.2916 11.8967L15.8686 15.149L21.4755 11.9109L15.8669 8.67182Z"
                                fill="currentColor"
                            />
                        </svg>
                    </template>
                    56F82SR21Q284
                    <v-tooltip activator="parent" location="top">
                        This project ID
                    </v-tooltip>
                </v-chip>
            </v-col>

            <v-col cols="12" md="4" class="d-flex justify-start justify-md-end align-top align-md-center">
                <v-btn>
                    Project Actions
                    <template #append>
                        <v-icon icon="mdi-chevron-down" />
                    </template>
                    <ProjectActionsMenu />
                </v-btn>
            </v-col>
        </v-row>

        <v-row>
            <v-col cols="12" md="4">
                <v-card title="Project" subtitle="My First Project" variant="flat" :border="true" rounded="xlg">
                    <v-card-text>
                        <v-chip color="success" variant="tonal" class="mr-2 font-weight-bold">
                            Pro
                            <v-tooltip activator="parent" location="top">
                                Pro account since: 2 May 2022
                            </v-tooltip>
                        </v-chip>
                        <v-divider class="my-4" />
                        <v-btn variant="outlined" size="small" color="default" class="mr-2">
                            Edit Project Information
                            <ProjectInformationDialog />
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" md="4">
                <v-card title="Value" subtitle="Attribution" variant="flat" :border="true" rounded="xlg" class="mb-3">
                    <v-card-text>
                        <v-chip color="default" variant="tonal" class="mr-2">Company</v-chip>
                        <v-divider class="my-4" />
                        <v-btn variant="outlined" size="small" color="default">
                            Set Value Attribution
                            <ProjectUserAgentsDialog />
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" md="4">
                <v-card title="Placement" subtitle="Region" variant="flat" :border="true" rounded="xlg">
                    <v-card-text>
                        <v-chip variant="tonal" class="mr-2">
                            Global
                        </v-chip>
                        <v-divider class="my-4" />
                        <v-btn variant="outlined" size="small" color="default">
                            Set Project Placement
                            <ProjectGeofenceDialog />
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>

        <v-row>
            <ProjectLimitsDialog />

            <v-col cols="12" sm="6" lg="4">
                <UsageProgressComponent
                    title="Buckets" :progress="5" used="5" percentage="5%" available="95" limit="100"
                    color="success" link
                />
            </v-col>

            <v-col cols="12" sm="6" lg="4">
                <UsageProgressComponent
                    title="Storage" :progress="14" used="141 TB" percentage="14%" available="859 TB"
                    limit="1,000 TB" color="success" link
                />
            </v-col>

            <v-col cols="12" sm="6" lg="4">
                <UsageProgressComponent
                    title="Download" :progress="59" used="590 TB" percentage="59%" available="310 TB"
                    limit="1,000 TB" color="success" link
                />
            </v-col>

            <v-col cols="12" sm="6" lg="4">
                <UsageProgressComponent
                    title="Segments" :progress="32" used="32,000" percentage="32%" available="68,000"
                    limit="100,000" color="success" link
                />
            </v-col>

            <v-col cols="12" sm="6" lg="4">
                <UsageProgressComponent
                    title="Rate" :progress="20" used="2,000" percentage="20%" available="8,000" limit="10,000"
                    color="success" link
                />
            </v-col>

            <v-col cols="12" sm="6" lg="4">
                <UsageProgressComponent
                    title="Burst" :progress="50" used="25,000" percentage="50%" available="25,000"
                    limit="50,000" color="success" link
                />
            </v-col>
        </v-row>

        <v-row>
            <v-col>
                <h3 class="my-4">Buckets</h3>
                <BucketsTableComponent />
            </v-col>
        </v-row>

        <v-row>
            <v-col>
                <h3 class="my-4">Users</h3>
                <UsersTableComponent />
            </v-col>
        </v-row>

        <v-row>
            <v-col>
                <h3 class="my-4">History</h3>
                <LogsTableComponent />
            </v-col>
        </v-row>
    </v-container>
</template>

<script lang="ts">
import {
    VContainer,
    VRow,
    VCol,
    VChip,
    VTooltip,
    VBtn,
    VIcon,
    VCard,
    VCardText,
    VDivider,
} from 'vuetify/components';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import UsageProgressComponent from '@/components/UsageProgressComponent.vue';
import BucketsTableComponent from '@/components/BucketsTableComponent.vue';
import LogsTableComponent from '@/components/LogsTableComponent.vue';
import UsersTableComponent from '@/components/UsersTableComponent.vue';
import ProjectActionsMenu from '@/components/ProjectActionsMenu.vue';
import ProjectGeofenceDialog from '@/components/ProjectGeofenceDialog.vue';
import ProjectUserAgentsDialog from '@/components/ProjectUserAgentsDialog.vue';
import ProjectLimitsDialog from '@/components/ProjectLimitsDialog.vue';
import ProjectInformationDialog from '@/components/ProjectInformationDialog.vue';

export default {
    components: {
        VContainer,
        VRow,
        VCol,
        VChip,
        VTooltip,
        VBtn,
        VIcon,
        VCard,
        VCardText,
        VDivider,
        PageTitleComponent,
        UsageProgressComponent,
        BucketsTableComponent,
        LogsTableComponent,
        UsersTableComponent,
        ProjectActionsMenu,
        ProjectGeofenceDialog,
        ProjectUserAgentsDialog,
        ProjectLimitsDialog,
        ProjectInformationDialog,
    },
    data() {
        return {
            dialog: false,
            valid: false,
        };
    },
    mounted() {
        document.title = 'Storj Admin - Project Details';
    },
};
</script>