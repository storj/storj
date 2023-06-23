// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="search-container">
        <SearchIcon class="search-container__icon" />
        <input
            v-model="searchQuery"
            class="search-container__input"
            placeholder="Search"
            type="text"
            autocomplete="off"
            readonly
            maxlength="72"
            @input="processSearchQuery"
            @focus="removeReadOnly"
            @blur="addReadOnly"
        >
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { useDOM } from '@/composables/DOM';

import SearchIcon from '@/../static/images/common/search.svg';

declare type searchCallback = (search: string) => Promise<void>;

const props = defineProps<{
    search: searchCallback,
}>();

const { removeReadOnly, addReadOnly } = useDOM();

const searchQuery = ref<string>('');

/**
 * Clears search query.
 */
function clearSearch(): void {
    searchQuery.value = '';
    processSearchQuery();
}

async function processSearchQuery(): Promise<void> {
    await props.search(searchQuery.value);
}

defineExpose({ clearSearch });
</script>

<style scoped lang="scss">
    .search-container {
        padding: 8px;
        display: flex;
        align-items: center;
        box-sizing: border-box;
        border: 1px solid var(--c-grey-3);
        border-radius: 10px;
        width: 250px;
        background-color: #fff;

        @media screen and (width <= 1150px) {
            width: 100%;
        }

        &__icon {
            margin: 0 12px 0 4px;
        }

        &__input {
            flex: 1;
            background-color: transparent;
            outline: none;
            border: none;
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            line-height: 20px;
        }
    }

    ::placeholder {
        color: var(--c-grey-6);
        opacity: 0.7;
    }
</style>
