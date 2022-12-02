// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header-container">
        <div class="header-container__buttons-area">
            <slot />
        </div>
        <div v-if="styleType === 'common'" class="search-container">
            <VSearch
                ref="search"
                :placeholder="placeholder"
                :search="search"
            />
        </div>
        <div v-if="styleType === 'access'">
            <VSearchAlternateStyling
                ref="search"
                :placeholder="placeholder"
                :search="search"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VSearch from '@/components/common/VSearch.vue';
import VSearchAlternateStyling from '@/components/common/VSearchAlternateStyling.vue';

declare type searchCallback = (search: string) => Promise<void>;

// @vue/component
@Component({
    components: {
        VSearch,
        VSearchAlternateStyling,
    },
})
export default class VHeader extends Vue {
    @Prop({ default: 'common' })
    private readonly styleType: string;
    @Prop({ default: '' })
    private readonly placeholder: string;
    @Prop({ default: function(): searchCallback {
        return async function(_: string) {};
    } })
    private readonly search: searchCallback;

    public $refs!: {
        search: VSearch;
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

        .search-container {
            position: relative;
        }
    }

    @media screen and (max-width: 1150px) {

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
