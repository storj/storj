// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div
        v-if="!appStore.state.settings"
        class="d-flex justify-center align-center align-items-center"
        style="height: 100vh;"
    >
        <v-skeleton-loader
            class="mx-auto"
            width="300"
            height="200"
            type="card"
        />
    </div>
    <template v-else>
        <router-view />
        <notifications />
    </template>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import { VSkeletonLoader } from 'vuetify/components';

import { useAppStore } from '@/store/app';
import { useNotificationsStore } from '@/store/notifications';
import { useUsersStore } from '@/store/users';

import Notifications from '@/layouts/default/Notifications.vue';

const appStore = useAppStore();
const usersStore = useUsersStore();
const notify = useNotificationsStore();

onMounted(async () => {
    try {
        await Promise.all([
            usersStore.getAccountFreezeTypes(),appStore.getSettings(),
            appStore.getPlacements(),
        ]);
    } catch (error) {
        notify.notifyError(`Failed to initialise app. ${error.message}`);
    }
});
</script>
