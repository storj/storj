// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="api-keys-header-container">
        <div class="api-keys-header-container__item">
            <SearchArea />
            <Button label="New API Key" width="240px" height="58px" :onPress="onDeleteButtonClick" />
        </div>
        <div class="api-keys-header-container__item">
            <SortApiKeysHeader />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import SearchArea from './SearchArea.vue';
import Button from '@/components/common/Button.vue';
import SortApiKeysHeader from '@/components/apiKeys/headerArea/SortApiKeysHeader.vue';

@Component({
    components: {
        SearchArea,
        Button,
        SortApiKeysHeader,
    }
})

export default class HeaderArea extends Vue {
}
</script>

<style scoped lang="scss">
    .api-keys-header-container {

        &__item {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &:last-child {
                padding-left: 20px;
            }
        }
    }
</style>