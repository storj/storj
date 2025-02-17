// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <slot v-bind="sessionTimeout" />

    <v-snackbar
        :model-value="sessionTimeout.debugTimerShown.value"
        :timeout="-1"
        color="warning"
        rounded="pill"
        min-width="0"
        location="top"
    >
        <v-icon :icon="Clock" />
        Remaining session time:
        <span class="font-weight-bold">{{ sessionTimeout.debugTimerText.value }}</span>
    </v-snackbar>

    <set-session-timeout-dialog v-model="isSetTimeoutModalShown" />
    <inactivity-dialog
        v-model="sessionTimeout.inactivityModalShown.value"
        :on-continue="() => sessionTimeout.refreshSession(true)"
        :on-logout="sessionTimeout.handleInactive"
    />
    <update-session-timeout-prompt-dialog
        v-model="isUpdateTimeoutPromptModalShown"
        @show-set-timeout-modal="isSetTimeoutModalShown = true"
    />
    <session-expired-dialog v-model="sessionTimeout.sessionExpiredModalShown.value" />
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { VSnackbar, VIcon } from 'vuetify/lib/components/index.mjs';
import { Clock } from 'lucide-vue-next';

import { useSessionTimeout } from '@/composables/useSessionTimeout';
import { LocalData } from '@/utils/localData';
import { useUsersStore } from '@/store/modules/usersStore';

import InactivityDialog from '@/components/dialogs/InactivityDialog.vue';
import SessionExpiredDialog from '@/components/dialogs/SessionExpiredDialog.vue';
import SetSessionTimeoutDialog from '@/components/dialogs/SetSessionTimeoutDialog.vue';
import UpdateSessionTimeoutPromptDialog from '@/components/dialogs/UpdateSessionTimeoutPromptDialog.vue';

const usersStore = useUsersStore();

const isSetTimeoutModalShown = ref<boolean>(false);
const isUpdateTimeoutPromptModalShown = ref<boolean>(false);

const sessionTimeout = useSessionTimeout({
    showEditSessionTimeoutModal: () => isSetTimeoutModalShown.value = true,
});

onMounted(() => {
    if (LocalData.getSessionHasExpired() && !usersStore.state.settings.sessionDuration) {
        isUpdateTimeoutPromptModalShown.value = true;
    }
});
</script>
