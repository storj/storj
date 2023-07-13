// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Access" />
        <PageSubtitleComponent subtitle="Create Access Grants, S3 Credentials, and API Keys." link="https://docs.storj.io/dcs/access" />

        <v-col>
            <v-row class="mt-2 mb-4">
                <v-btn>
                    <!-- <svg width="16" height="16" viewBox="0 0 18 18" fill="none" class="mr-2" xmlns="http://www.w3.org/2000/svg">
                <path d="M4.83987 16.8886L1.47448 17.099C1.17636 17.1176 0.919588 16.891 0.900956 16.5929C0.899551 16.5704 0.899551 16.5479 0.900956 16.5254L1.11129 13.16C1.11951 13.0285 1.17546 12.9045 1.26864 12.8114L5.58927 8.49062L5.57296 8.43619C4.98999 6.44548 5.49345 4.26201 6.96116 2.72323L7.00936 2.67328L7.05933 2.62271C9.35625 0.325796 13.0803 0.325796 15.3772 2.62271C17.6741 4.91963 17.6741 8.64366 15.3772 10.9406C13.8503 12.4674 11.6456 13.0112 9.62856 12.4455L9.56357 12.4269L9.50918 12.4107L5.18856 16.7313C5.09538 16.8244 4.97139 16.8804 4.83987 16.8886ZM2.45229 15.5477L4.38997 15.4266L9.13372 10.6827L9.58862 10.864C11.2073 11.5091 13.072 11.1424 14.3255 9.88889C16.0416 8.17281 16.0416 5.39048 14.3255 3.6744C12.6094 1.95831 9.8271 1.95831 8.11101 3.6744C6.87177 4.91364 6.49924 6.7502 7.11424 8.3559L7.13584 8.41118L7.31711 8.86605L2.57342 13.61L2.45229 15.5477ZM10.7858 7.21411C11.3666 7.79494 12.3083 7.79494 12.8892 7.21411C13.47 6.63328 13.47 5.69157 12.8892 5.11074C12.3083 4.52991 11.3666 4.52991 10.7858 5.11074C10.205 5.69157 10.205 6.63328 10.7858 7.21411Z" fill="currentColor"/>
                </svg> -->
                    <svg width="16" height="16" class="mr-2" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M10 1C14.9706 1 19 5.02944 19 10C19 14.9706 14.9706 19 10 19C5.02944 19 1 14.9706 1 10C1 5.02944 5.02944 1 10 1ZM10 2.65C5.94071 2.65 2.65 5.94071 2.65 10C2.65 14.0593 5.94071 17.35 10 17.35C14.0593 17.35 17.35 14.0593 17.35 10C17.35 5.94071 14.0593 2.65 10 2.65ZM10.7496 6.8989L10.7499 6.91218L10.7499 9.223H12.9926C13.4529 9.223 13.8302 9.58799 13.8456 10.048C13.8602 10.4887 13.5148 10.8579 13.0741 10.8726L13.0608 10.8729L10.7499 10.873L10.75 13.171C10.75 13.6266 10.3806 13.996 9.925 13.996C9.48048 13.996 9.11807 13.6444 9.10066 13.2042L9.1 13.171L9.09985 10.873H6.802C6.34637 10.873 5.977 10.5036 5.977 10.048C5.977 9.60348 6.32857 9.24107 6.76882 9.22366L6.802 9.223H9.09985L9.1 6.98036C9.1 6.5201 9.46499 6.14276 9.925 6.12745C10.3657 6.11279 10.7349 6.45818 10.7496 6.8989Z" fill="currentColor" />
                    </svg>

                    New Access

                    <v-dialog
                        v-model="dialog"
                        activator="parent"
                        min-width="400px"
                        width="auto"
                        transition="fade-transition"
                        scrollable
                    >
                        <v-card rounded="xlg">
                            <v-sheet>
                                <v-card-item class="pl-7 py-4">
                                    <template #prepend>
                                        <v-card-title class="font-weight-bold">
                                            <!-- <v-icon>
                                                <img src="../assets/icon-team.svg" alt="Team">
                                            </v-icon> -->
                                            {{ stepTitles[step] }}
                                        </v-card-title>
                                    </template>

                                    <template #append>
                                        <v-btn
                                            icon="$close"
                                            variant="text"
                                            size="small"
                                            color="default"
                                            @click="dialog = false"
                                        />
                                    </template>
                                </v-card-item>
                            </v-sheet>

                            <v-divider />

                            <v-window v-model="step">
                                <v-window-item :value="0">
                                    <v-form class="pa-8 pb-3">
                                        <v-row>
                                            <v-col cols="12">
                                                <!-- <h4 class="mb-2">Name</h4> -->
                                                <v-text-field
                                                    label="Access Name"
                                                    placeholder="Enter name for this access"
                                                    variant="outlined"
                                                    color="default"
                                                    autofocus
                                                />
                                            </v-col>

                                            <v-col>
                                                <h4 class="mb-2">Type</h4>
                                                <v-checkbox color="primary" density="compact">
                                                    <template #label>
                                                        <span class="mx-2">Access Grant</span>
                                                        <span>
                                                            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" tabindex="0"><path d="M8 15.2A7.2 7.2 0 118 .8a7.2 7.2 0 010 14.4zm0-1.32A5.88 5.88 0 108 2.12a5.88 5.88 0 000 11.76zm-.6-3.42V8.343a.66.66 0 011.32-.026V10.416c0 .368-.292.67-.66.682a.639.639 0 01-.66-.638zm0-4.92v-.077a.66.66 0 011.32-.026v.059c0 .368-.292.67-.66.682a.639.639 0 01-.66-.638z" fill="currentColor" /></svg>
                                                            <v-tooltip activator="parent" location="top">
                                                                <span>Keys to upload, delete, and view your data. Learn more</span>
                                                            </v-tooltip>
                                                        </span>
                                                    </template>
                                                </v-checkbox>
                                                <v-checkbox color="primary" density="compact">
                                                    <template #label>
                                                        <span class="mx-2">S3 Credentials</span>
                                                        <span>
                                                            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" tabindex="0"><path d="M8 15.2A7.2 7.2 0 118 .8a7.2 7.2 0 010 14.4zm0-1.32A5.88 5.88 0 108 2.12a5.88 5.88 0 000 11.76zm-.6-3.42V8.343a.66.66 0 011.32-.026V10.416c0 .368-.292.67-.66.682a.639.639 0 01-.66-.638zm0-4.92v-.077a.66.66 0 011.32-.026v.059c0 .368-.292.67-.66.682a.639.639 0 01-.66-.638z" fill="currentColor" /></svg>
                                                            <v-tooltip activator="parent" location="top">
                                                                <span>Generates access key, secret key, and endpoint to use in your S3 supported application. Learn More</span>
                                                            </v-tooltip>
                                                        </span>
                                                    </template>
                                                </v-checkbox>
                                                <v-checkbox color="primary" density="compact">
                                                    <template #label>
                                                        <span class="mx-2">CLI Access</span>
                                                        <span>
                                                            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" tabindex="0"><path d="M8 15.2A7.2 7.2 0 118 .8a7.2 7.2 0 010 14.4zm0-1.32A5.88 5.88 0 108 2.12a5.88 5.88 0 000 11.76zm-.6-3.42V8.343a.66.66 0 011.32-.026V10.416c0 .368-.292.67-.66.682a.639.639 0 01-.66-.638zm0-4.92v-.077a.66.66 0 011.32-.026v.059c0 .368-.292.67-.66.682a.639.639 0 01-.66-.638z" fill="currentColor" /></svg>
                                                            <v-tooltip activator="parent" location="top">
                                                                <span>Create an access grant to run in the command line. Learn more</span>
                                                            </v-tooltip>
                                                        </span>
                                                    </template>
                                                </v-checkbox>
                                            </v-col>
                                        </v-row>
                                    </v-form>
                                </v-window-item>

                                <v-window-item :value="1">
                                    <v-form class="pa-8 pb-3">
                                        <v-row>
                                            <v-col cols="12">
                                                <p>Permissions</p>
                                                <p>Buckets</p>
                                                <p>End date</p>
                                            </v-col>
                                        </v-row>
                                    </v-form>
                                </v-window-item>

                                <v-window-item :value="2">
                                    <v-form class="pa-8 pb-3">
                                        <v-row>
                                            <v-col cols="12">
                                                <v-text-field
                                                    label="Password"
                                                    type="password"
                                                    variant="outlined"
                                                />
                                                <v-text-field
                                                    label="Confirm Password"
                                                    type="password"
                                                    variant="outlined"
                                                />
                                            </v-col>
                                        </v-row>
                                    </v-form>
                                </v-window-item>
                            </v-window>

                            <v-divider />

                            <v-card-actions class="pa-7">
                                <v-row>
                                    <v-col>
                                        <v-btn
                                            v-if="!step"
                                            variant="outlined"
                                            color="default"
                                            block
                                        >
                                            Learn More
                                        </v-btn>
                                        <v-btn
                                            v-else
                                            variant="outlined"
                                            color="default"
                                            block
                                            @click="step--"
                                        >
                                            Back
                                        </v-btn>
                                    </v-col>
                                    <v-col>
                                        <v-btn
                                            v-if="step < 2"
                                            color="primary"
                                            variant="flat"
                                            block
                                            @click="step++"
                                        >
                                            Next
                                        </v-btn>
                                    </v-col>
                                </v-row>
                            </v-card-actions>
                        </v-card>
                    </v-dialog>
                </v-btn>
            </v-row>
        </v-col>

        <AccessTableComponent />
    </v-container>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VContainer,
    VCol,
    VRow,
    VBtn,
    VDialog,
    VCard,
    VSheet,
    VCardItem,
    VCardTitle,
    VDivider,
    VWindow,
    VWindowItem,
    VForm,
    VTextField,
    VCheckbox,
    VTooltip,
    VCardActions,
} from 'vuetify/components';

import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@poc/components/PageSubtitleComponent.vue';
import AccessTableComponent from '@poc/components/AccessTableComponent.vue';

const dialog = ref<boolean>(false);
const step = ref<number>(0);

const stepTitles = [
    'Create New Access',
    'Permissions',
    'Passphrase',
    'Access Created',
];
</script>
