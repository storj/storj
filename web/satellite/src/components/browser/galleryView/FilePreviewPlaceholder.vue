// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="file-preview-placeholder">
        <p class="file-preview-placeholder__key">{{ file?.Key || '' }}</p>
        <p class="file-preview-placeholder__label">No preview available</p>
        <VButton
            icon="download"
            :label="`Download (${prettyBytes(file?.Size || 0)})`"
            :on-press="() => emit('download')"
            width="188px"
            height="52px"
            border-radius="10px"
            font-size="14px"
        />
    </div>
</template>

<script setup lang="ts">
import prettyBytes from 'pretty-bytes';

import { BrowserObject } from '@/store/modules/objectBrowserStore';

import VButton from '@/components/common/VButton.vue';

const props = defineProps<{
    file: BrowserObject;
}>();

const emit = defineEmits<{
    download: [];
}>();
</script>

<style scoped lang="scss">
.file-preview-placeholder {
    width: 100%;
    align-self: center;
    display: flex;
    align-items: center;
    flex-direction: column;

    &__key {
        font-size: 16px;
        line-height: 24px;
        color: var(--c-white);
        margin-bottom: 14px;
    }

    &__label {
        font-family: 'font_bold', sans-serif;
        font-size: 28px;
        line-height: 36px;
        letter-spacing: -0.02em;
        color: var(--c-white);
        margin-bottom: 17px;
    }
}
</style>
