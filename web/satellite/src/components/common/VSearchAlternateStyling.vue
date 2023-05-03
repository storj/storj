// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <input
        v-model="searchQuery"
        class="access-search-input"
        :placeholder="`Search ${placeholder}`"
        type="text"
        autocomplete="off"
        readonly
        @input="processSearchQuery"
        @focus="removeReadOnly"
        @blur="addReadOnly"
    >
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { useDOM } from '@/composables/DOM';

declare type searchCallback = (search: string) => Promise<void>;

const props = withDefaults(defineProps<{
    placeholder?: string,
    search: searchCallback,
}>(), { placeholder: '' });

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
    .access-search-input {
        position: absolute;
        left: 0;
        bottom: 0;
        padding: 0 10px 0 50px;
        box-sizing: border-box;
        outline: none;
        border: 1px solid var(--c-grey-3);
        border-radius: 10px;
        height: 40px;
        width: 250px;
        font-family: 'font_regular', sans-serif;
        font-size: 16px;
        background-color: #fff;
        background-image: url('../../../static/images/common/search-gray.png');
        background-repeat: no-repeat;
        background-size: 22px 22px;
        background-position: top 8px left 14px;

        @media screen and (max-width: 1150px) {
            width: 100%;
        }
    }

    ::placeholder {
        color: #afb7c1;
    }
</style>
