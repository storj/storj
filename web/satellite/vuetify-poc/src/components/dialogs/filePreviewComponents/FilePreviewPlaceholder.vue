// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="h-100 w-100 mt-n16 d-flex flex-column align-center justify-center">
        <p class="mb-5">{{ file?.Key || '' }}</p>
        <p class="text-h5 mb-5 font-weight-bold">No preview available</p>
        <v-btn
            @click="() => emits('download')"
        >
            <template #prepend>
                <img src="@poc/assets/icon-download.svg" width="22" alt="Download">
            </template>
            {{ `Download (${prettyBytes(file?.Size || 0)})` }}
        </v-btn>
    </div>
</template>

<script setup lang="ts">
import { VBtn } from 'vuetify/components';
import prettyBytes from 'pretty-bytes';

import { BrowserObject } from '@/store/modules/objectBrowserStore';

const props = defineProps<{
  file: BrowserObject,
}>();

const emits = defineEmits<{
    download: [];
}>();
</script>
