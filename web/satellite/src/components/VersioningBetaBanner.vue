// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        v-if="alertVisible"
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
            <versioning-beta-dialog v-model="dialogVisible" />
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
import { ref, watch } from 'vue';
import { mdiArrowRight, mdiClose } from '@mdi/js';

import { useLoading } from '@/composables/useLoading';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';

import VersioningBetaDialog from '@/components/dialogs/VersioningBetaDialog.vue';

const projectsStore = useProjectsStore();
const usersStore = useUsersStore();

const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const alertVisible = ref(false);
const dialogVisible = ref(false);

/**
 * Dismiss the versioning beta banner.
 */
function closeAlert() {
    withLoading(async () => {
        try {
            const noticeDismissal = { ...usersStore.state.settings.noticeDismissal };
            noticeDismissal.versioningBetaBanner = true;
            await usersStore.updateSettings({ noticeDismissal });
            alertVisible.value = false;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.VERSIONING_BETA_BANNER);
        }
    });
}

watch(() => [projectsStore.promptForVersioningBeta, dialogVisible.value], (values) => {
    if (values[0] && !alertVisible.value) {
        alertVisible.value = true;
    } else if (!values[0] && !values[1] && alertVisible.value) {
        // throttle the banner dismissal for the dialog close animation.
        setTimeout(() => alertVisible.value = false, 500);
    }
}, { immediate: true });
</script>
