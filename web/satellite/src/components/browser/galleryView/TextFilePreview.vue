// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VLoader v-if="isLoading" class="text-file-preview__loader" width="100px" height="100px" is-white />
    <div v-else-if="!isError" class="text-file-preview__container">
        <div class="text-file-preview__container__content">{{ content }}</div>
    </div>
    <slot v-else />
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import VLoader from '@/components/common/VLoader.vue';

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const props = defineProps<{
    src: string;
}>();

const content = ref<string>('');
const isError = ref<boolean>(false);

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

<style scoped lang="scss">
.text-file-preview {

    &__container {
        width: 100%;
        height: 100%;
        padding: 16px;
        box-sizing: border-box;
        overflow-y: auto;

        &__content {
            width: 100%;
            min-height: 100%;
            padding: 16px;
            box-sizing: border-box;
            background-color: var(--c-white);
            font-family: monospace;
            font-size: 16px;
            word-break: break-word;
        }
    }

    &__loader {
        align-items: center;
    }
}
</style>
