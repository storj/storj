// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        v-if="shouldBeShown"
        v-model="visible"
        closable
        variant="outlined"
        type="info"
        class="my-4 pb-4"
        border
        @click:close="onClose"
    >
        <template #title>
            <!-- eslint-disable-next-line vue/no-v-html -->
            <p v-html="announcementCfg.title" />
        </template>
        <!-- eslint-disable-next-line vue/no-v-html -->
        <p v-html="announcementCfg.body" />
    </v-alert>
</template>

<script setup lang="ts">
import { VAlert } from 'vuetify/components';
import { computed, ref } from 'vue';

import { AnnouncementConfig } from '@/types/config.gen';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';

const notify = useNotify();

const configStore = useConfigStore();
const userStore = useUsersStore();

const visible = ref<boolean>(true);

const announcementCfg = computed<AnnouncementConfig>(() => configStore.state.config.announcement);

const shouldBeShown = computed<boolean>(() => {
    if (!announcementCfg.value.enabled) return false;

    const settings = userStore.state.settings;
    if (!settings.noticeDismissal.announcements) return true;

    const val = settings.getAnnouncementStatus(announcementCfg.value.name);
    if (val === undefined) return true;

    return !val;
});

async function onClose(): Promise<void> {
    try {
        const noticeDismissal = { ...userStore.state.settings.noticeDismissal };
        if (!noticeDismissal.announcements) noticeDismissal.announcements = {};

        noticeDismissal.announcements[announcementCfg.value.name] = true;
        await userStore.updateSettings({ noticeDismissal });
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ANNOUNCEMENT_BANNER);
    }
}
</script>

<style scoped lang="scss">
p {
    color: rgb(var(--v-theme-on-background)) !important;
}
</style>
