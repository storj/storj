// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-snackbar
        v-model="doNotificationsExist"
        absolute
        top
        right
        class="custom-snackbar"
    >
        <NotificationItem
            v-for="item in notifications"
            :key="item.id"
            :item="item"
        />
    </v-snackbar>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VSnackbar } from 'vuetify/components';

import NotificationItem from './NotificationItem.vue';

import { DelayedNotification } from '@/app/types/delayedNotification';
import { useNotificationsStore } from '@/app/store/notificationsStore';

const notificationsStore = useNotificationsStore();

const doNotificationsExist = ref<boolean>(false);

const notifications = computed<DelayedNotification[]>(() => notificationsStore.state.notificationQueue as DelayedNotification[]);
const hasNotifications = computed<boolean>(() => notifications.value.length > 0);

watch(hasNotifications, (newValue: boolean) => {
    doNotificationsExist.value = newValue;
}, { immediate: true });
</script>

<style lang="scss" scoped>
.custom-snackbar {

    :deep(.v-snack__content) {
        margin-right: -9px;
    }

    .v-alert {
        margin: 10px;
    }

    :deep(.v-snack__wrapper.theme--dark) {
        background-color: transparent;
        color: rgb(255 255 255 / 87%);
    }

    :deep(.v-alert__icon.v-icon) {
        top: 12px;
    }

    :deep(.v-sheet.v-snack__wrapper:not(.v-sheet--outlined)) {
        box-shadow: none;
    }
}
</style>
