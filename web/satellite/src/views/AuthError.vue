// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height" fluid>
        <v-row justify="center" align="center">
            <v-col cols="12" sm="9" md="7" lg="5" xl="4" xxl="3">
                <v-card class="pa-2 pa-sm-7">
                    <h2 class="mb-3">Authentication Failed</h2>
                    <p>Please <a class="link font-weight-bold" :href="supportUrl" target="_blank" rel="noopener noreferrer">contact support</a> if issue persists.</p>
                    <v-btn
                        v-if="primaryAuthLoginURL"
                        :href="primaryAuthLoginURL"
                        color="primary"
                        size="large"
                        class="mt-6"
                        block
                    >
                        Try Again
                    </v-btn>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
    VBtn,
    VCard,
    VCol,
    VContainer,
    VRow,
} from 'vuetify/components';

import { useConfigStore } from '@/store/modules/configStore';

const configStore = useConfigStore();
const primaryAuthLoginURL = computed(() => configStore.state.config.primaryAuthLoginURL);
const supportUrl = computed(() => configStore.supportUrl);
</script>
