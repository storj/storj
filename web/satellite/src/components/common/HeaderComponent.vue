// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header-container">
        <h1>{{title}}</h1>
        <div class="header-container__wrapper">
            <div class="header-container__buttons-area">
                <slot></slot>
            </div>
            <SearchComponent ref="search" :placeHolder="placeHolder" :search="search"/>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';
    import SearchComponent from '@/components/common/SearchComponent.vue';

    declare type searchCallback = (search: string) => Promise<void>;
    declare interface ClearSearch {
        clearSearch: () => void;
    }

    @Component({
        components: {
            SearchComponent,
        }
    })
    export default class HeaderComponent extends Vue {
        @Prop({default: ''})
        private readonly placeHolder: string;
        @Prop({default: () => { return ''; }})
        private readonly search: searchCallback;
        @Prop({default: ''})
        private readonly title;

        public $refs!: {
            search: SearchComponent & ClearSearch;
        };

        public clearSearch() {
            this.$refs.search.clearSearch();
        }
    }
</script>

<style scoped lang="scss">
    .header-container {
        width: 100%;

        h1 {
            font-family: 'font_bold';
            font-size: 32px;
            line-height: 39px;
            margin: 0;
        }

        &__wrapper {
            width: 100%;
            height: 85px;
            position: relative;
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-top: 10px;

            &__buttons-area {
                width: auto;
                display: flex;
                align-items: center;
                justify-content: space-between;
            }
        }
    }
</style>
