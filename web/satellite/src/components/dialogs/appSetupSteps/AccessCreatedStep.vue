// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-7">
        <v-row>
            <v-col>
                <p class="font-weight-bold mb-2">Access Created</p>
                <p>Copy and save the access credentials for this application as they will only appear once.</p>
            </v-col>

            <v-col cols="12">
                <v-text-field
                    flat
                    density="comfortable"
                    label="Access Key"
                    :model-value="credentials.accessKeyId"
                    readonly
                    hide-details
                >
                    <template #append-inner>
                        <input-copy-button :value="credentials.accessKeyId" />
                    </template>
                </v-text-field>
            </v-col>
            <v-col cols="12">
                <v-text-field
                    flat
                    density="comfortable"
                    label="Secret Key"
                    :model-value="credentials.secretKey"
                    readonly
                    hide-details
                >
                    <template #append-inner>
                        <input-copy-button :value="credentials.secretKey" />
                    </template>
                </v-text-field>
            </v-col>
            <v-col cols="12">
                <v-text-field
                    flat
                    density="comfortable"
                    label="Endpoint"
                    :model-value="credentials.endpoint"
                    readonly
                    hide-details
                >
                    <template #append-inner>
                        <input-copy-button :value="credentials.endpoint" />
                    </template>
                </v-text-field>
            </v-col>

            <v-col>
                <v-alert variant="tonal" color="info">
                    <p class="text-subtitle-2 font-weight-bold">Next steps</p>
                    <p class="text-subtitle-2">Please read the documentation to find where to enter the access you created.</p>
                </v-alert>
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { VForm, VAlert, VCol, VRow, VTextField } from 'vuetify/components';

import { EdgeCredentials } from '@/types/accessGrants';

import InputCopyButton from '@/components/InputCopyButton.vue';

defineProps<{
    credentials: EdgeCredentials
}>();
</script>
