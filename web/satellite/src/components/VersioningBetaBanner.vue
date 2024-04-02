// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        :model-value="alertVisible"
        class="my-4 pb-4"
        variant="tonal"
        color="default"
        title="Object Versioning Beta Available"
        border
        closable
        @click:close="closeAlert"
    >
        <p class="text-body-2 mt-2 mb-3">Choose if you want to take part of the beta, and enable versioning for this project.</p>
        <v-btn
            color="default"
            :append-icon="mdiArrowRight"
            class="mr-3"
            :disabled="isLoading"
        >
            Learn More
            <versioning-beta-dialog />
        </v-btn>
        <v-btn
            color="default"
            :append-icon="mdiClose"
            :loading="isLoading"
            @click="closeAlert"
        >
            Dismiss
        </v-btn>
    </v-alert>
</template>

<script setup lang="ts">
import { VAlert, VBtn } from 'vuetify/components';
import { computed } from 'vue';
import { mdiArrowRight, mdiClose } from '@mdi/js';

import { useLoading } from '@/composables/useLoading';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';

import VersioningBetaDialog from '@/components/dialogs/VersioningBetaDialog.vue';

const projectsStore = useProjectsStore();

const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const alertVisible = computed<boolean>(() => projectsStore.promptForVersioningBeta);

// dismiss this alert and automatically opt out of versioning.
function closeAlert() {
    withLoading(async () => {
        try {
            await projectsStore.setVersioningOptInStatus('out');
            await projectsStore.getProjectConfig();
            await projectsStore.getProjects();
        } catch (e) {
            notify.notifyError(e, AnalyticsErrorEventSource.VERSIONING_BETA_BANNER);
        }
    });
}
</script>
