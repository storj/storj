// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header-container">
        <div class="header-container__buttons-area">
            <slot />
        </div>
        <VSearch
            ref="search"
            :placeholder="placeholder"
            :search="search"
            :style-type="styleType"
        />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VSearch from '@/components/common/VSearch.vue';

declare type searchCallback = (search: string) => Promise<void>;
declare interface ClearSearch {
    clearSearch(): void;
}

// @vue/component
@Component({
    components: {
        VSearch,
    },
})
export default class VHeader extends Vue {
    @Prop({default: 'common'})
    private readonly styleType: string;
    @Prop({default: ''})
    private readonly placeholder: string;
    @Prop({default: () => ''})
    private readonly search: searchCallback;

    public $refs!: {
        search: VSearch & ClearSearch;
    };

    public clearSearch(): void {
        this.$refs.search.clearSearch();
    }
}
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
    }
</style>
