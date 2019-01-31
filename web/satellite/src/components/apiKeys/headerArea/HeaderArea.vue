// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="api-keys-header-container">
        <div class="api-keys-header-container__item">
            <SearchArea />
            <Button label="New API Key" width="240px" height="54px" :onPress="toggleSelection" />
        </div>
        <div class="api-keys-header-container__item">
            <!-- <SortApiKeysHeader /> -->
        </div>
        <AddApiKeyPopup v-if="isPopupShown" @onClose="toggleSelection" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import SearchArea from './SearchArea.vue';
import Button from '@/components/common/Button.vue';
import SortApiKeysHeader from '@/components/apiKeys/headerArea/SortApiKeysHeader.vue';
import AddApiKeyPopup from '@/components/apiKeys/AddApiKeyPopup.vue';

@Component({
    data: function() {
        return {
            isPopupShown: false,
        };
    },
    components: {
        SearchArea,
        Button,
        SortApiKeysHeader,
        AddApiKeyPopup,
    },
    methods: {
        toggleSelection: function () {
            this.$data.isPopupShown = !this.$data.isPopupShown;
        }
    }
})

export default class HeaderArea extends Vue {
}
</script>

<style scoped lang="scss">
    .api-keys-header-container {

        margin-bottom: 4px;

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