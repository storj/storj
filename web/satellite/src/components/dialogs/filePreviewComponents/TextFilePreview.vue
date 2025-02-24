// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="isLoading" class="w-100 h-100 d-flex align-center justify-center">
        <v-progress-circular indeterminate />
    </div>
    <v-container v-else-if="!isError" class="w-100 h-100 overflow-y-auto">
        <v-sheet
            :theme="isDark ? 'dark' : 'light'"
            :color="isDark ? 'rgba(0, 0, 0, 0.3)' : undefined"
            :border="isDark"
            class="w-100 pa-6 rounded-lg mx-auto break-word"
            max-width="calc(100% - 144px)"
            min-height="100%"
        >
            <code>{{ content }}</code>
        </v-sheet>
    </v-container>
    <slot v-else />
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useTheme } from 'vuetify';
import { VProgressCircular, VContainer, VSheet } from 'vuetify/components';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { APIError } from '@/utils/error';

const theme = useTheme();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const props = defineProps<{
    src: string;
}>();

const content = ref<string>('');
const isError = ref<boolean>(false);

const isDark = computed(() => theme.global.current.value.dark);

onMounted(async () => {
    await withLoading(async () => {
        try {
            const response = await fetch(props.src);
            const text = await response.text();
            if (!response.ok) {
                let message = 'Failed to preview';
                if (text.includes('limit exceeded')) {
                    message = `${message}, bandwidth limit exceeded`;
                }
                notify.notifyError(new APIError({ message, status: response.status, requestID: null }), AnalyticsErrorEventSource.GALLERY_VIEW);
                isError.value = true;
                return;
            }
            content.value = text;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.GALLERY_VIEW);
            isError.value = true;
        }
    });
});
</script>
