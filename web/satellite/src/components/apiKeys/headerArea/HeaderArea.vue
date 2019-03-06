// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="api-keys-header-container">
        <div class="api-keys-header-container__item">
            <SearchArea />
            <div id="addApiKeyPopupButton">
                <Button label="New API Key" width="240px" height="54px" :onPress="togglePopup" />
            </div>
        </div>
        <div class="api-keys-header-container__item">
            <!-- <SortApiKeysHeader /> -->
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import SearchArea from './SearchArea.vue';
import Button from '@/components/common/Button.vue';
import SortApiKeysHeader from '@/components/apiKeys/headerArea/SortApiKeysHeader.vue';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        SearchArea,
        Button,
        SortApiKeysHeader,
    },
    methods: {
        togglePopup: function () {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_API_KEY);
        },
    },
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