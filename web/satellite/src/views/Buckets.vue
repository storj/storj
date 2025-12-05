// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <trial-expiration-banner v-if="isTrialExpirationBanner && isUserProjectOwner" :expired="isExpired" />

        <PageTitleComponent
            title="Browse Buckets"
            extra-info="Project usage statistics are not real-time. Recent uploads, downloads, or other actions may not be immediately reflected."
        />
        <PageSubtitleComponent
            subtitle="Buckets are where you upload and organize your data."
            link="https://docs.storj.io/learn/concepts/key-architecture-constructs#bucket"
        />

        <v-row class="mt-1 mb-2">
            <v-col>
                <v-btn :prepend-icon="CirclePlus" @click="onCreateBucket">
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
import { CirclePlus } from 'lucide-vue-next';

import { usePreCheck } from '@/composables/usePreCheck';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import BucketsDataTable from '@/components/BucketsDataTable.vue';
import CreateBucketDialog from '@/components/dialogs/CreateBucketDialog.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';

const { isTrialExpirationBanner, isUserProjectOwner, isExpired, withTrialCheck, withManagedPassphraseCheck } = usePreCheck();

const isCreateBucketDialogOpen = ref<boolean>(false);

/**
 * Starts create bucket flow if user's free trial is not expired.
 */
function onCreateBucket(): void {
    withTrialCheck(() => { withManagedPassphraseCheck(() => {
        isCreateBucketDialogOpen.value = true;
    });});
}
</script>
