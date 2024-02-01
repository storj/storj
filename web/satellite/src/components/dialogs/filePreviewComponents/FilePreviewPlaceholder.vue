// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="h-100 w-100 d-flex flex-column align-center justify-center">
        <p class="mb-5">{{ file?.Key ?? '' }}</p>
        <p class="text-h5 mb-5 font-weight-bold">No preview available</p>
        <v-btn
            @click="() => emits('download')"
        >
            <template #prepend>
                <img src="@/assets/icon-download.svg" width="22" alt="Download">
            </template>
            {{ `Download (${formattedSize})` }}
        </v-btn>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VBtn } from 'vuetify/components';

import { BrowserObject } from '@/store/modules/objectBrowserStore';
import { Size } from '@/utils/bytesSize';

const props = defineProps<{
    file: BrowserObject,
}>();

const emits = defineEmits<{
    download: [];
}>();

const formattedSize = computed<string>(() => {
    const size = new Size(props.file?.Size ?? 0);
    return `${size.formattedBytes} ${size.label}`;
});
</script>
