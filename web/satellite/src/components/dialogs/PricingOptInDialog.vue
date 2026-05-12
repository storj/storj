// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        fullscreen
        persistent
        transition="fade-transition"
    >
        <v-sheet color="surface" height="100%" class="d-flex align-center justify-center pa-6">
            <div style="max-width: 680px; width: 100%;" class="text-center">
                <!-- Logo -->
                <div class="mb-5">
                    <icon-storj-logo
                        height="50"
                        width="50"
                        class="rounded-xlg bg-background pa-2 border"
                    />
                </div>

                <!-- Effective date -->
                <p class="text-label-medium text-medium-emphasis mb-1">
                    Effective July 1, 2026
                </p>

                <!-- Headline -->
                <h2 class="text-headline-medium font-weight-bold mb-2">
                    Your storage plan is changing.
                </h2>

                <!-- Subtitle -->
                <p class="text-body-medium text-medium-emphasis mb-6">
                    We are simplifying to two storage tiers. Here's exactly what changes for your account.
                </p>

                <!-- Pricing card -->
                <v-card variant="outlined" rounded="lg" class="text-left mb-6">
                    <v-card-text class="pa-6">
                        <p class="font-weight-bold text-body-large mb-1">Global &amp; Archive</p>
                        <p class="text-body-medium text-medium-emphasis mb-3">
                            Automatically migrating on July 1, 2026 to the new plan:
                        </p>
                        <p class="text-title-large font-weight-bold text-primary mb-4">Standard</p>
                        <div class="d-flex flex-column ga-3">
                            <div v-for="feature in standardFeatures" :key="feature" class="d-flex align-center ga-2">
                                <v-icon :icon="Check" size="16" />
                                <span class="text-body-medium">{{ feature }}</span>
                            </div>
                        </div>
                    </v-card-text>
                </v-card>

                <!-- Footer text -->
                <p class="text-body-medium text-medium-emphasis mb-6">
                    Clicking Accept &amp; Continue confirms you've reviewed these changes.<br>
                    Have questions? Read the
                    <a href="https://storj.io/pricing" target="_blank" rel="noopener noreferrer" class="text-primary">pricing FAQ</a>
                    or
                    <a href="https://storj.io/contact-sales" target="_blank" rel="noopener noreferrer" class="text-primary">contact support</a>.
                </p>

                <!-- Actions -->
                <div class="d-flex align-center justify-center ga-3 flex-wrap">
                    <v-btn
                        variant="outlined"
                        color="default"
                        size="large"
                        :loading="isOptingOut"
                        :disabled="isOptingIn"
                        @click="onDecline"
                    >
                        I want to opt out
                    </v-btn>
                    <v-btn
                        color="primary"
                        size="large"
                        :append-icon="ArrowRight"
                        :loading="isOptingIn"
                        :disabled="isOptingOut"
                        @click="onOptIn"
                    >
                        Accept &amp; Continue
                    </v-btn>
                </div>
            </div>
        </v-sheet>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VBtn, VCard, VCardText, VDialog, VIcon, VImg, VSheet } from 'vuetify/components';
import { ArrowRight, Check } from 'lucide-vue-next';
import { useTheme } from 'vuetify';

import { OptInStatus } from '@/types/users';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import IconStorjLogo from '@/components/icons/IconStorjLogo.vue';

const usersStore = useUsersStore();
const appStore = useAppStore();
const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const notify = useNotify();
const theme = useTheme();

const model = defineModel<boolean>({ required: true });

const isOptingIn = ref(false);
const isOptingOut = ref(false);

const standardFeatures = [
    'Storage: $7/TB per month',
    'Egress: $7/TB',
    'Storage locations: Global distribution',
    'Object Mount included 2 seats free',
];

async function onOptIn(): Promise<void> {
    if (isOptingIn.value || isOptingOut.value) return;
    isOptingIn.value = true;
    try {
        await usersStore.updateSettings({ optInStatus: OptInStatus.OptedIn });
        appStore.dismissPricingOptInDialog();
    } catch (error) {
        notify.notifyError(error);
    } finally {
        isOptingIn.value = false;
    }
}

async function onDecline(): Promise<void> {
    if (isOptingIn.value || isOptingOut.value) return;
    isOptingOut.value = true;
    try {
        await usersStore.updateSettings({ optInStatus: OptInStatus.OptedOut });
        appStore.dismissPricingOptInDialog();
    } catch (error) {
        notify.notifyError(error);
    } finally {
        isOptingOut.value = false;
    }
}
</script>
