// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        closable
        variant="tonal"
        type="info"
        rounded="lg"
        class="mt-4 mb-2"
        title="Team Info"
        text="All team members should use the same passphrase to access the same data."
        @click:close="onDismiss"
    />
</template>

<script setup lang="ts">
import { VAlert } from 'vuetify/components';

import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const userStore = useUsersStore();

const notify = useNotify();

/**
 * Handles on alert dismiss logic.
 */
async function onDismiss(): Promise<void> {
    try {
        const noticeDismissal = { ...userStore.state.settings.noticeDismissal };
        noticeDismissal.projectMembersPassphrase = true;
        await userStore.updateSettings({ noticeDismissal });
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_DASHBOARD_PAGE);
    }
}
</script>
