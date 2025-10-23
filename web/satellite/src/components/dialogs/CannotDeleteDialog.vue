// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="460px"
        transition="fade-transition"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="TriangleAlert" :size="18" color="orange" />
                    </v-sheet>
                </template>

                <v-card-title class="font-weight-bold">
                    Cannot Delete {{ isBucket ? 'Bucket' : 'Access Key' }}
                </v-card-title>

                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-text class="font-weight-bold pb-0">
                You can only delete {{ isBucket ? 'buckets' : 'access keys' }} that you created.
            </v-card-text>

            <v-card-item>
                <v-alert color="default" variant="tonal" width="auto">
                    <p class="text-subtitle-2">
                        {{ isBucket ? bucket?.name : access?.name }}
                        <br><br>
                        This {{ isBucket ? 'bucket' : 'access key' }} is owned by:
                        <br>
                        {{ isBucket ? (bucket?.creatorEmail || 'unknown') : (access?.creatorEmail || 'unknown') }}
                    </p>
                </v-alert>
            </v-card-item>

            <v-card-text>
                To delete this {{ isBucket ? 'bucket' : 'access key' }}, you'll need to contact the owner or a project admin.
            </v-card-text>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn color="default" variant="outlined" block @click="model = false">Cancel</v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VCardText,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VSheet,
    VAlert,
} from 'vuetify/components';
import { TriangleAlert, X } from 'lucide-vue-next';

import { Bucket } from '@/types/buckets';
import { AccessGrant } from '@/types/accessGrants';

const props = defineProps<{
    bucket?: Bucket,
    access?: AccessGrant,
}>();

const model = defineModel<boolean>({ required: true });

const isBucket = computed<boolean>(() => props.bucket !== undefined);
</script>
