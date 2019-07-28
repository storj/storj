// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="api-keys-header-container">
        <div class="api-keys-header-container__item">
            <div id="addApiKeyPopupButton">
                <Button label="New API Key" width="170px" height="56px" :onPress="togglePopup" />
            </div>
            <SearchArea />
        </div>
        <div class="api-keys-header-container__item">
            <!-- <SortApiKeysHeader /> -->
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import SearchArea from './SearchArea.vue';
    import Button from '../../common/Button.vue';
    import SortApiKeysHeader from '@/components/apiKeys/headerArea/SortApiKeysHeader.vue';
    import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

    @Component({
        components: {
            SearchArea,
            Button,
        }
    })
    export default class HeaderArea extends Vue {
        public togglePopup(): void {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_API_KEY);
        }
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
