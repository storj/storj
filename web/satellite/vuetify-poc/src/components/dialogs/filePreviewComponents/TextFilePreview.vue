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
            class="w-100 pa-4 mx-auto break-word"
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
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

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
            content.value = await (await fetch(props.src)).text();
        } catch (error) {
            notify.error(`Error fetching object. ${error.message}`, AnalyticsErrorEventSource.GALLERY_VIEW);
            isError.value = true;
        }
    });
});
</script>
