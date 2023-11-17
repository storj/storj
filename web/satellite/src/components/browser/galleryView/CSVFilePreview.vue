// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VLoader v-if="isLoading" class="csv-file-preview__loader" width="100px" height="100px" is-white />
    <div v-else-if="!isError" class="csv-file-preview__container">
        <table>
            <tr v-for="(row, rowIdx) in items" :key="rowIdx">
                <td v-for="(col, colIdx) in row" :key="colIdx">
                    {{ col }}
                </td>
            </tr>
        </table>
    </div>
    <slot v-else />
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import Papa, { ParseResult } from 'papaparse';

import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import VLoader from '@/components/common/VLoader.vue';

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
                notify.error(`Error parsing object. ${error.message}`, AnalyticsErrorEventSource.GALLERY_VIEW);
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
.csv-file-preview {

    &__container {
        width: 100%;
        max-height: 100%;
        padding: 16px;
        box-sizing: border-box;
        overflow: auto;
        align-self: flex-start;

        table {
            min-width: 100%;
            border-collapse: collapse;

            td {
                background: var(--c-white);
                border: 1px solid var(--c-grey-3);
                padding: 6px 10px;
                white-space: nowrap;
            }
        }
    }

    &__loader {
        align-items: center;
    }
}
</style>
