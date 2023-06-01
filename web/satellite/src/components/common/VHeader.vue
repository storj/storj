// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header-container">
        <div class="header-container__buttons-area">
            <slot />
        </div>
        <div v-if="styleType === 'common'" class="search-container">
            <VSearch
                ref="searchInput"
                :placeholder="placeholder"
                :search="search"
            />
        </div>
        <div v-if="styleType === 'access'">
            <VSearchAlternateStyling
                ref="searchInput"
                :placeholder="placeholder"
                :search="search"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import VSearch from '@/components/common/VSearch.vue';
import VSearchAlternateStyling from '@/components/common/VSearchAlternateStyling.vue';

type searchCallback = (search: string) => Promise<void>;

const props = withDefaults(defineProps<{
    placeholder: string;
    search: searchCallback;
    styleType?: string;
}>(), {
    placeholder: '',
    styleType: 'common',
});

const searchInput = ref<{ clearSearch: () => void }>();

function clearSearch(): void {
    searchInput.value?.clearSearch();
}

defineExpose({ clearSearch });
</script>

<style scoped lang="scss">
    .header-container {
        width: 100%;
        height: 85px;
        position: relative;
        display: flex;
        align-items: center;
        justify-content: space-between;

        &__buttons-area {
            width: auto;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }

        .search-container {
            position: relative;
        }
    }

    @media screen and (width <= 1150px) {

        .header-container {
            flex-direction: column;
            align-items: flex-start;
            margin-bottom: 75px;

            .search-container {
                width: 100%;
                margin-top: 30px;
            }
        }
    }
</style>
