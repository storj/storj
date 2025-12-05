// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-col cols="12" md="6" lg="6" xl="3">
        <v-card class="px-1 py-3 h-100 align-content-space-between flex-grow-1 d-flex flex-column">
            <v-card-item class="pb-0">
                <img
                    v-if="app"
                    :src="app.src"
                    :alt="app.name"
                    width="48"
                    height="48"
                    class="rounded-md border pa-2 bg-background mr-3"
                >
                <div v-else class="d-inline-flex align-center justify-center border rounded icon-wrap">
                    <PlusIcon />
                </div>
            </v-card-item>

            <v-card-item>
                <h3 class="mb-2">
                    {{ app?.name ?? 'Don\'t see your app?' }}
                </h3>
                <p class="text-high-emphasis">
                    {{ app?.description ?? 'Create S3 credentials that work with any S3 compatible app.' }}
                </p>
            </v-card-item>

            <v-card-item class="bottom mt-auto">
                <v-btn color="default" variant="outlined" :append-icon="ArrowRight" @click="onSetup">
                    Setup
                </v-btn>
                <v-btn
                    v-if="app && app.docs"
                    variant="outlined"
                    color="default"
                    class="ml-2"
                    :href="app.docs"
                    :append-icon="SquareArrowOutUpRight"
                    target="_blank"
                    rel="noopener noreferrer"
                    @click="() => sendAnalytics(AnalyticsEvent.APPLICATIONS_DOCS_CLICKED)"
                >
                    Docs
                </v-btn>
            </v-card-item>
        </v-card>
    </v-col>
    <AccessSetupDialog
        v-model="dialog"
        :app="app"
        :access-name="app?.name"
        :docs-link="app?.docs ?? 'https://docs.storj.io/dcs/access'"
        :default-step="app ? SetupStep.ChooseFlowStep : SetupStep.ChooseAccessStep"
        :default-access-type="neededAccessType"
    />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VBtn, VCard, VCardItem, VCol } from 'vuetify/components';
import { ArrowRight, SquareArrowOutUpRight, PlusIcon } from 'lucide-vue-next';

import { Application, UplinkApp } from '@/types/applications';
import { AccessType, SetupStep } from '@/types/setupAccess';
import { usePreCheck } from '@/composables/usePreCheck';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import AccessSetupDialog from '@/components/dialogs/AccessSetupDialog.vue';

const props = defineProps<{
    app?: Application
}>();

const analyticsStore  = useAnalyticsStore();

const { withTrialCheck, withManagedPassphraseCheck } = usePreCheck();

const dialog = ref<boolean>(false);

/**
 * Returns access type needed for current app.
 */
const neededAccessType = computed<AccessType>(() => {
    return props.app?.name === UplinkApp.name ? AccessType.AccessGrant : AccessType.S3;
});

/**
 * Holds on setup button click logic.
 * Starts create S3 credentials flow.
 */
function onSetup(): void {
    withTrialCheck(() => { withManagedPassphraseCheck(() => {
        sendAnalytics(AnalyticsEvent.APPLICATIONS_SETUP_CLICKED);
        dialog.value = true;
    });});
}

function sendAnalytics(e: AnalyticsEvent): void {
    analyticsStore.eventTriggered(e, props.app && { application: props.app.name });
}
</script>

<style scoped lang="scss">
.icon-wrap {
    width: 48px;
    height: 48px;
}
</style>
