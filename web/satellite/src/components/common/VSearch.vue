// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <input
        ref="input"
        v-model="searchQuery"
        readonly
        class="common-search-input"
        :placeholder="`Search ${placeholder}`"
        :style="style"
        type="text"
        autocomplete="off"
        @mouseenter="onMouseEnter"
        @mouseleave="onMouseLeave"
        @input="processSearchQuery"
        @focus="removeReadOnly"
        @blur="addReadOnly"
    >
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { useDOM } from '@/composables/DOM';

type searchCallback = (search: string) => Promise<void>;
interface SearchStyle {
    width: string;
}

const props = withDefaults(defineProps<{
    search: searchCallback;
    placeholder?: string;
}>(), {
    placeholder: '',
});

const { removeReadOnly, addReadOnly } = useDOM();

const inputWidth = ref<string>('56px');
const searchQuery = ref<string>('');
const input = ref<HTMLInputElement>();

const style = computed((): SearchStyle => {
    return { width: inputWidth.value };
});

/**
 * Expands search input.
 */
function onMouseEnter(): void {
    inputWidth.value = '540px';
    input.value?.focus();
}

/**
 * Collapses search input if no search query.
 */
function onMouseLeave(): void {
    if (!searchQuery.value) {
        inputWidth.value = '56px';
        input.value?.blur();
    }
}

/**
 * Clears search query and collapses input.
 */
function clearSearch(): void {
    searchQuery.value = '';
    processSearchQuery();
    inputWidth.value = '56px';
}

async function processSearchQuery(): Promise<void> {
    await props.search(searchQuery.value);
}

defineExpose({ clearSearch });
</script>

<style scoped lang="scss">
    .common-search-input {
        position: absolute;
        right: 0;
        bottom: 50%;
        transform: translateY(50%);
        padding: 0 38px 0 18px;
        border: 1px solid #f2f2f2;
        box-sizing: border-box;
        box-shadow: 0 4px 4px rgb(231 232 238 / 60%);
        outline: none;
        border-radius: 36px;
        height: 56px;
        font-family: 'font_regular', sans-serif;
        font-size: 16px;
        transition: all 0.4s ease-in-out;
        background-image: url('../../../static/images/common/search.png');
        background-repeat: no-repeat;
        background-size: 22px 22px;
        background-position: top 16px right 16px;
    }

    @media screen and (width <= 1150px) {

        .common-search-input {
            width: 100% !important;
        }
    }
</style>
