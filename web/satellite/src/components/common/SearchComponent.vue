// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <input
        @mouseenter="onMouseEnter"
        @mouseleave="onMouseLeave"
        @input="onInput"
        v-model="searchQuery"
        :placeholder="`Search ${placeHolder}`"
        :style="customWidth"
        type="text">
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';

    declare type searchCallback = (search: string) => Promise<void>;

    @Component
    export default class SearchComponent extends Vue {
        @Prop({default: ''})
        private readonly placeHolder: string;
        @Prop({default: () => { return ''; }})
        private readonly search: searchCallback;

        private inputWidth: string = '56px';
        private searchQuery: string = '';

        public onMouseEnter(): void {
            this.inputWidth = '602px';
        }

        public onMouseLeave(): void {
            if (!this.searchQuery) {
                this.inputWidth = '56px';
            }
        }

        public onInput(): any {
            this.onMouseLeave();
            this.processSearchQuery();
        }

        public get customWidth(): object {
            return { width: this.inputWidth };
        }

        public async processSearchQuery() {
            await this.search(this.searchQuery);
        }
    }
</script>

<style scoped lang="scss">
    input {
        position: absolute;
        right: 0;
        padding: 0 38px 0 18px;
        border: 1px solid #F2F2F2;
        box-sizing: border-box;
        box-shadow: 0 4px 4px rgba(231, 232, 238, 0.6);
        outline: none;
        border-radius: 36px;
        height: 56px;
        font-family: 'font_regular';
        font-size: 16px;
        transition: all 0.4s ease-in-out;
        background-image: url('../../../static/images/team/searchIcon.svg');
        background-repeat: no-repeat;
        background-size: 22px 22px;
        background-position: top 16px right 16px;
    }

    ::-webkit-input-placeholder {
        color: #AFB7C1;
    }
</style>
