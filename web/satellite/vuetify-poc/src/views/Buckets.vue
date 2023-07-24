// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Buckets" />
        <PageSubtitleComponent subtitle="Buckets are storage containers for your data." link="https://docs.storj.io/dcs/buckets" />

        <v-row class="mt-2 mb-4">
            <v-col>
                <v-btn
                    color="primary"
                >
                    <svg width="16" height="16" class="mr-2" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M10 1C14.9706 1 19 5.02944 19 10C19 14.9706 14.9706 19 10 19C5.02944 19 1 14.9706 1 10C1 5.02944 5.02944 1 10 1ZM10 2.65C5.94071 2.65 2.65 5.94071 2.65 10C2.65 14.0593 5.94071 17.35 10 17.35C14.0593 17.35 17.35 14.0593 17.35 10C17.35 5.94071 14.0593 2.65 10 2.65ZM10.7496 6.8989L10.7499 6.91218L10.7499 9.223H12.9926C13.4529 9.223 13.8302 9.58799 13.8456 10.048C13.8602 10.4887 13.5148 10.8579 13.0741 10.8726L13.0608 10.8729L10.7499 10.873L10.75 13.171C10.75 13.6266 10.3806 13.996 9.925 13.996C9.48048 13.996 9.11807 13.6444 9.10066 13.2042L9.1 13.171L9.09985 10.873H6.802C6.34637 10.873 5.977 10.5036 5.977 10.048C5.977 9.60348 6.32857 9.24107 6.76882 9.22366L6.802 9.223H9.09985L9.1 6.98036C9.1 6.5201 9.46499 6.14276 9.925 6.12745C10.3657 6.11279 10.7349 6.45818 10.7496 6.8989Z" fill="currentColor" />
                    </svg>

                    New Bucket

                    <v-dialog
                        v-model="dialog"
                        activator="parent"
                        width="auto"
                        min-width="400px"
                        transition="fade-transition"
                    >
                        <v-card rounded="xlg">
                            <v-sheet>
                                <v-card-item class="pl-7 py-4">
                                    <template #prepend>
                                        <v-card-title class="font-weight-bold">
                                            <!-- <img src="../assets/icon-bucket-color.svg" alt="Bucket" width="40"> -->
                                            Create New Bucket
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

                            <v-form v-model="valid" class="pa-8 pb-3">
                                <v-row>
                                    <v-col>
                                        <p>Buckets are used to store and organize your files.</p>

                                        <v-text-field
                                            v-model="bucketName"
                                            variant="outlined"
                                            :rules="bucketNameRules"
                                            label="Enter bucket name"
                                            hint="Lowercase letters, numbers, hyphens (-), and periods (.)"
                                            required
                                            autofocus
                                            class="mt-8 mb-3"
                                        />
                                    </v-col>
                                </v-row>
                            </v-form>

                            <v-divider />

                            <v-card-actions class="pa-7">
                                <v-row>
                                    <v-col>
                                        <v-btn variant="outlined" color="default" block @click="dialog = false">Cancel</v-btn>
                                    </v-col>
                                    <v-col>
                                        <v-btn color="primary" variant="flat" block>
                                            Create Bucket
                                        </v-btn>
                                    </v-col>
                                </v-row>
                            </v-card-actions>
                        </v-card>
                    </v-dialog>
                </v-btn>
            </v-col>
        </v-row>

        <BucketsDataTable />
    </v-container>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VContainer,
    VRow,
    VCol,
    VBtn,
    VDialog,
    VCard,
    VSheet,
    VCardItem,
    VCardTitle,
    VDivider,
    VForm,
    VTextField,
    VCardActions,
} from 'vuetify/components';

import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@poc/components/PageSubtitleComponent.vue';
import BucketsDataTable from '@poc/components/BucketsDataTable.vue';

const dialog = ref<boolean>(false);
const bucketName = ref<string>('');

const bucketNameRules = [
    (value: string) => (!!value || 'Bucket name is required.'),
    (value: string) => {
        if (/^[a-z0-9-.]+$/.test(value)) return true;
        if (/[A-Z]/.test(value)) return 'Uppercase characters are not allowed.';
        if (/\s/.test(value)) return 'Spaces are not allowed.';
        if (/[^a-zA-Z0-9-.]/.test(value)) return 'Other characters are not allowed.';
        return true;
    },
];
</script>
