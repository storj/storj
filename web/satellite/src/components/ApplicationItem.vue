// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-col cols="12" md="6" lg="6" xl="3">
        <v-card class="px-2 py-4 h-100 align-content-space-between">
            <v-card-item class="pb-0">
                <img :src="app.src" :alt="app.name" width="48" height="48" class="rounded">
            </v-card-item>

            <v-card-item>
                <v-chip size="small" variant="tonal" color="default" class="mb-2">
                    {{ app.category }}
                </v-chip>
                <h3 class="mb-1">
                    {{ app.name }}
                </h3>
                <p class="mt-1 text-high-emphasis">
                    {{ app.description }}
                </p>
            </v-card-item>

            <v-card-item class="bottom">
                <v-btn color="primary" @click="onSetup">
                    Setup
                    <template #append>
                        <v-icon :icon="mdiArrowRight" />
                    </template>
                </v-btn>
                <v-btn
                    variant="outlined"
                    color="default"
                    class="ml-2"
                    :href="app.docs"
                    target="_blank"
                    rel="noopener noreferrer"
                    @click="() => sendAnalytics(AnalyticsEvent.APPLICATIONS_DOCS_CLICKED)"
                >
                    Docs
                    <template #append>
                        <v-icon :icon="mdiOpenInNew" />
                    </template>
                </v-btn>
            </v-card-item>
        </v-card>
    </v-col>
    <AccessSetupDialog
        v-if="newAppSetupFlowEnabled"
        v-model="dialog"
        :app="app"
        :access-name="app.name"
        :docs-link="app.docs"
        :default-access-type="neededAccessType"
        is-app-setup
    />
    <CreateAccessDialog v-else ref="accessDialog" v-model="dialog" :default-name="app.name" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VBtn, VCard, VCardItem, VChip, VCol, VIcon } from 'vuetify/components';
import { mdiArrowRight, mdiOpenInNew } from '@mdi/js';

import { Application, UplinkApp } from '@/types/applications';
import { AccessType, Exposed } from '@/types/createAccessGrant';
import { useTrialCheck } from '@/composables/useTrialCheck';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import CreateAccessDialog from '@/components/dialogs/CreateAccessDialog.vue';
import AccessSetupDialog from '@/components/dialogs/AccessSetupDialog.vue';

const props = defineProps<{
    app: Application
}>();

const { newAppSetupFlowEnabled } = useConfigStore().state.config;
const analyticsStore  = useAnalyticsStore();

const { withTrialCheck } = useTrialCheck();

const accessDialog = ref<Exposed>();
const dialog = ref<boolean>(false);

/**
 * Returns access type needed for current app.
 */
const neededAccessType = computed<AccessType>(() => {
    return props.app.name === UplinkApp.name ? AccessType.AccessGrant : AccessType.S3;
});

/**
 * Holds on setup button click logic.
 * Starts create S3 credentials flow.
 */
function onSetup(): void {
    withTrialCheck(() => {
        sendAnalytics(AnalyticsEvent.APPLICATIONS_SETUP_CLICKED);
        accessDialog.value?.setTypes([neededAccessType.value]);
        dialog.value = true;
    });
}

function sendAnalytics(e: AnalyticsEvent): void {
    analyticsStore.eventTriggered(e, { application: props.app.name });
}
</script>
