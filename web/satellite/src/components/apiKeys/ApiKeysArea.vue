// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div v-if="apiKeyItems > 0" class="api-keys-header">
            <HeaderArea/>
        </div>
        <div v-if="apiKeyItems > 0" class="api-keys-container">
            <div class="api-keys-container__content">
                <ApiKeysItem />
                <ApiKeysItem />
                <ApiKeysItem />
                <ApiKeysItem />
                <ApiKeysItem />
                <ApiKeysItem />
            </div>
            <PaginationArea />
        </div>
        <EmptyState
            v-if="apiKeyItems === 0"
            mainTitle="You have no API Keys yet"
            :imageSource="emptyImage"
            buttonLabel="New Api Key"
            isButtonShown />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import EmptyState from '@/components/common/EmptyStateArea.vue';
import HeaderArea from '@/components/apiKeys/headerArea/HeaderArea.vue';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
import ApiKeysItem from '@/components/apiKeys/ApiKeysItem.vue';
import PaginationArea from '@/components/apiKeys/PaginationArea.vue';

@Component({
    data: function () {
        return {
            emptyImage: EMPTY_STATE_IMAGES.API_KEY,
            apiKeyItems: 1,
        };
    },
    methods: {},
    computed: {},
    components: {
        EmptyState,
        HeaderArea,
        ApiKeysItem,
        PaginationArea,
    }
})

export default class ApiKeysArea extends Vue {
}
</script>

<style scoped lang="scss">
    .api-keys-header {
        padding: 44px 40px 0 40px;
    }

    .api-keys-container {
        padding: 0px 40px 0 60px;
    }
</style>