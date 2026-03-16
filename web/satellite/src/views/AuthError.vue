// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-app>
        <auth-bar />
        <v-main>
            <v-container class="fill-height" fluid>
                <v-row justify="center" align="center">
                    <v-col cols="12" sm="9" md="7" lg="5" xl="4" xxl="3">
                        <v-card class="pa-2 pa-sm-7">
                            <h2 class="mb-3">{{ title }}</h2>
                            <p>{{ message }}</p>
                            <p class="mt-3">Please <a class="link font-weight-bold" :href="supportUrl" target="_blank" rel="noopener noreferrer">contact support</a> if the issue persists.</p>
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
                            <v-btn
                                v-else
                                color="primary"
                                size="large"
                                class="mt-6"
                                block
                                @click="router.push(ROUTES.Login.path)"
                            >
                                Try Again
                            </v-btn>
                        </v-card>
                    </v-col>
                </v-row>
            </v-container>
        </v-main>
    </v-app>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
    VApp,
    VBtn,
    VCard,
    VCol,
    VContainer,
    VMain,
    VRow,
} from 'vuetify/components';

import { useConfigStore } from '@/store/modules/configStore';
import { ROUTES } from '@/router';

import AuthBar from '@/layouts/auth/AuthBar.vue';

const configStore = useConfigStore();
const route = useRoute();
const router = useRouter();

const isRateLimited = computed(() => route.path === ROUTES.RateLimited.path);
const title = computed(() => isRateLimited.value ? 'Too Many Requests' : 'Authentication Failed');
const message = computed(() => isRateLimited.value
    ? 'You\'ve exceeded your request limit. Please wait a moment and try again.'
    : 'An error occurred during authentication.',
);
const primaryAuthLoginURL = computed(() => configStore.state.config.primaryAuthLoginURL);
const supportUrl = computed(() => configStore.supportUrl);
</script>
