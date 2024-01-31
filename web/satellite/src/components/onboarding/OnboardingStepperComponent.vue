// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row>
        <v-col cols="12" md="6" lg="3" xl="3">
            <v-card class="pa-5 pt-3">
                <p class="text-overline">
                    Step 1
                </p>
                <h4>
                    Encryption Passphrase
                </h4>
                <p class="mt-1 mb-2">
                    This passphrase will be used to
                    encrypt all your data in this project.
                </p>
                <v-btn :prepend-icon="mdiCheck" disabled variant="tonal" color="default" block>
                    Set a Passphrase
                </v-btn>
            </v-card>
        </v-col>
        <v-col cols="12" md="6" lg="3" xl="3">
            <v-card class="pa-5 pt-3">
                <p class="text-overline">
                    Step 2
                </p>
                <h4>
                    Create a Bucket
                </h4>
                <p class="mt-1 mb-2">
                    Buckets are used to store and
                    organize your data in this project.
                </p>
                <v-btn :prepend-icon="mdiCheck" disabled variant="tonal" color="default" block>
                    Create a Bucket
                </v-btn>
            </v-card>
        </v-col>
        <v-col cols="12" md="6" lg="3" xl="3">
            <v-card class="pa-5 pt-3">
                <p class="text-overline">
                    Step 3
                </p>
                <h4>
                    Upload Files
                </h4>
                <p class="mt-1 mb-2">
                    You are ready to upload files
                    in your bucket, and share with the world.
                </p>
                <v-btn
                    router-link
                    to="/bucket"
                    block
                >
                    <template #prepend>
                        <IconUpload />
                    </template>
                    Upload Files
                </v-btn>
            </v-card>
        </v-col>
        <v-col cols="12" md="6" lg="3" xl="3">
            <v-card class="pa-5 pt-3">
                <p class="text-overline">
                    Optional
                </p>
                <h4>
                    S3 Credentials
                </h4>
                <p class="mt-1 mb-2">
                    Connect your S3 compatible application
                    to Storj with S3 credentials.
                </p>
                <v-btn :append-icon="mdiArrowRight" variant="outlined" color="default" block router-link to="/apps">
                    Create Access Key
                </v-btn>
            </v-card>
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCol, VRow } from 'vuetify/components';
import { mdiArrowRight, mdiCheck } from '@mdi/js';

import IconUpload from '@/components/icons/IconUpload.vue';
</script>