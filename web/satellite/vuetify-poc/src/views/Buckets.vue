// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Browse Buckets" />
        <PageSubtitleComponent
            subtitle="Buckets are where you upload and organize your data."
            link="https://docs.storj.io/learn/concepts/key-architecture-constructs#bucket"
        />

        <v-row class="mt-2 mb-4">
            <v-col>
                <v-btn
                    color="primary"
                    @click="isCreateBucketDialogOpen = true"
                >
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

import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@poc/components/PageSubtitleComponent.vue';
import BucketsDataTable from '@poc/components/BucketsDataTable.vue';
import CreateBucketDialog from '@poc/components/dialogs/CreateBucketDialog.vue';
import IconNew from '@poc/components/icons/IconNew.vue';

const isCreateBucketDialogOpen = ref<boolean>(false);
</script>
