// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <trial-expiration-banner v-if="isTrialExpirationBanner" :expired="isExpired" />

        <PageTitleComponent title="Browse Buckets" />
        <PageSubtitleComponent
            subtitle="Buckets are where you upload and organize your data."
            link="https://docs.storj.io/learn/concepts/key-architecture-constructs#bucket"
        />

        <v-row class="mt-2 mb-4">
            <v-col>
                <v-btn color="primary" @click="onCreateBucket">
                    <IconNew class="mr-2" size="16" bold />
                    New Bucket
                </v-btn>
            </v-col>
        </v-row>

        <BucketsDataTable />
    </v-container>

    <CreateBucketDialog v-model="isCreateBucketDialogOpen" />
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VContainer,
    VRow,
    VCol,
    VBtn,
} from 'vuetify/components';

import { useTrialCheck } from '@/composables/useTrialCheck';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import BucketsDataTable from '@/components/BucketsDataTable.vue';
import CreateBucketDialog from '@/components/dialogs/CreateBucketDialog.vue';
import IconNew from '@/components/icons/IconNew.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';

const { isTrialExpirationBanner, isExpired, withTrialCheck } = useTrialCheck();

const isCreateBucketDialogOpen = ref<boolean>(false);

/**
 * Starts create bucket flow if user's free trial is not expired.
 */
function onCreateBucket(): void {
    withTrialCheck(() => {
        isCreateBucketDialogOpen.value = true;
    });
}
</script>
