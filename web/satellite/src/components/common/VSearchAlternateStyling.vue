// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <input
        v-model="searchQuery"
        class="access-search-input"
        :placeholder="`Search ${placeholder}`"
        type="text"
        autocomplete="off"
        @input="processSearchQuery"
    >
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

declare type searchCallback = (search: string) => Promise<void>;

// @vue/component
@Component
export default class VSearch extends Vue {
    @Prop({ default: '' })
    private readonly placeholder: string;
    @Prop({ default: function(): searchCallback {
        return async function(_: string) {};
    } })
    private readonly search: searchCallback;
    private searchQuery = '';

    public $refs!: {
        input: HTMLElement;
    };

    public get searchString(): string {
        return this.searchQuery;
    }

    /**
     * Clears search query.
     */
    public clearSearch(): void {
        this.searchQuery = '';
        this.processSearchQuery();
    }

    public async processSearchQuery(): Promise<void> {
        await this.search(this.searchQuery);
    }
}
</script>

<style scoped lang="scss">
    .access-search-input {
        position: absolute;
        left: 0;
        bottom: 0;
        padding: 0 10px 0 50px;
        box-sizing: border-box;
        outline: none;
        border: 1px solid #d8dee3;
        border-radius: 10px;
        height: 56px;
        width: 250px;
        font-family: 'font_regular', sans-serif;
        font-size: 16px;
        background-color: #fff;
        background-image: url('../../../static/images/common/search-gray.png');
        background-repeat: no-repeat;
        background-size: 22px 22px;
        background-position: top 16px left 16px;
    }

    ::placeholder {
        color: #afb7c1;
    }
</style>
