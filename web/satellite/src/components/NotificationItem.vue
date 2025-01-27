// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        closable
        variant="elevated"
        :title="item.title || item.type"
        :type="(item.type.toLowerCase() as 'error' | 'success' | 'warning' | 'info')"
        class="my-2"
        border
        @mouseover="() => onMouseOver(item.id)"
        @mouseleave="() => onMouseLeave(item.id)"
        @click:close="() => onCloseClick(item.id)"
    >
        <template #default>
            <p ref="messageArea">
                <component :is="item.messageNode" />
            </p>
            <a v-if="isSupportLinkMentioned" class="d-inline-block mt-2 white-link" :href="requestURL" target="_blank" rel="noopener noreferrer">
                Contact Support
            </a>
        </template>
    </v-alert>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { VAlert } from 'vuetify/components';

import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { DelayedNotification } from '@/types/DelayedNotification';
import { useConfigStore } from '@/store/modules/configStore';

const notificationsStore = useNotificationsStore();
const configStore = useConfigStore();

defineProps<{
    item: DelayedNotification
}>();

const isSupportLinkMentioned = ref<boolean>(false);
const messageArea = ref<HTMLParagraphElement>();

/**
 * Returns the URL for the general request page from the store.
 */
const requestURL = computed((): string => {
    return configStore.state.config.generalRequestURL;
});

/**
 * Forces notification to stay on page on mouse over it.
 */
function onMouseOver(id: string): void {
    notificationsStore.pauseNotification(id);
}

/**
 * Resume notification flow when mouse leaves notification.
 */
function onMouseLeave(id: string): void {
    notificationsStore.resumeNotification(id);
}

/**
 * Removes notification when the close button is clicked.
 */
function onCloseClick(id: string): void {
    notificationsStore.deleteNotification(id);
}

onMounted(() => {
    const msg = messageArea.value?.innerText.toLowerCase() || '';
    isSupportLinkMentioned.value = msg.includes('support');
});
</script>
