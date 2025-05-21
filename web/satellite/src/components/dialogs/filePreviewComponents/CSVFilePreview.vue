// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="isLoading" class="w-100 h-100 d-flex align-center justify-center">
        <v-progress-circular indeterminate />
    </div>
    <v-container v-else-if="!isError" class="w-100 max-h-100 overflow-auto">
        <table :class="`v-theme--${theme.global.name.value}`">
            <tr v-for="(row, rowIdx) in items" :key="rowIdx">
                <td v-for="(col, colIdx) in row" :key="colIdx">
                    {{ col }}
                </td>
            </tr>
        </table>
    </v-container>
    <slot v-else />
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useTheme } from 'vuetify';
import { VProgressCircular, VContainer } from 'vuetify/components';
import Papa, { ParseResult } from 'papaparse';

import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const theme = useTheme();
const notify = useNotify();

const props = defineProps<{
    src: string;
}>();

const items = ref<string[][]>([]);
const isLoading = ref<boolean>(true);
const isError = ref<boolean>(false);

onMounted(() => {
    try {
        Papa.parse(props.src, {
            download: true,
            worker: true,
            header: false,
            skipEmptyLines: true,
            complete: (results: ParseResult<string[]>) => {
                if (results) items.value = results.data;
                isLoading.value = false;
            },
            error: (error: Error) => {
                if (isError.value) return;
                const message = error.message.includes('Forbidden') ? 'Bandwidth limit exceeded' : 'Failed to preview';
                notify.error(`Error parsing object. ${message}`, AnalyticsErrorEventSource.GALLERY_VIEW);
                isError.value = true;
            },
        });
    } catch (error) {
        notify.error(`Error parsing object. ${error.message}`, AnalyticsErrorEventSource.GALLERY_VIEW);
        isError.value = true;
    }
});
</script>

<style scoped lang="scss">
table {
    min-width: 100%;
    color: rgb(var(--v-theme-on-background));
    border-collapse: collapse;

    td {
        background: rgb(var(--v-theme-surface));
        padding: 6px 10px;
        white-space: nowrap;

        /* stylelint-disable-next-line color-function-notation */
        border: 1px solid rgb(var(--v-border-color),var(--v-border-opacity));
    }
}
</style>
