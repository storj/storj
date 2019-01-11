// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div v-if="!isEmpty" class="api-keys-header">
            <HeaderArea/>
        </div>
        <div v-if="!isEmpty" class="api-keys-container">
            <div class="api-keys-container__content">
                <div v-for="apiKey in apiKeys" v-on:click="toggleSelection(apiKey.id)">
                    <ApiKeysItem
                            v-bind:class="[apiKey.isSelected ? 'selected': null]"
                            :apiKey="apiKey" />
                </div>
            </div>
            <Footer v-if="isSelected"/>
        </div>
        <EmptyState
            :onButtonClick="togglePopup"
            v-if="isEmpty"
            mainTitle="You have no API Keys yet"
            :imageSource="emptyImage"
            buttonLabel="New Api Key"
            isButtonShown />
        <AddAPIKeyPopup v-if="isPopupShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import EmptyState from '@/components/common/EmptyStateArea.vue';
import HeaderArea from '@/components/apiKeys/headerArea/HeaderArea.vue';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
import ApiKeysItem from '@/components/apiKeys/ApiKeysItem.vue';
import AddAPIKeyPopup from '@/components/apiKeys/AddApiKeyPopup.vue';
import Footer from '@/components/apiKeys/footerArea/Footer.vue';
import { API_KEYS_ACTIONS, APP_STATE_ACTIONS } from "@/utils/constants/actionNames";


@Component({
    data: function () {
        return {
            emptyImage: EMPTY_STATE_IMAGES.API_KEY,
        };
    },
    methods: {
        toggleSelection: function(id: string): void {
            this.$store.dispatch(API_KEYS_ACTIONS.TOGGLE_SELECTION, id);
        },
        togglePopup: function (): void {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_API_KEY);
        },
    },
    computed: {
        apiKeys: function (): any {
            return this.$store.state.apiKeysModule.apiKeys;
        },
        isEmpty: function (): boolean {
            return this.$store.state.apiKeysModule.apiKeys.length === 0;
        },
        isSelected: function (): boolean {
            return this.$store.getters.selectedAPIKeys.length > 0;
        },
        isPopupShown: function (): boolean {
            return this.$store.state.appStateModule.appState.isNewAPIKeyPopupShown;
        }
    },
    components: {
        EmptyState,
        HeaderArea,
        ApiKeysItem,
        AddAPIKeyPopup,
        Footer
    },
})

export default class ApiKeysArea extends Vue {
}
</script>

<style scoped lang="scss">
    // .api-keys-header {
    //     padding: 44px 40px 0 40px;
    // }

    // .api-keys-container {
    //     padding: 0px 40px 0 60px;
    // }
    .api-keys-header {
        position: fixed;
        padding: 55px 30px 25px 64px;
        max-width: 79.7%;
        width: 100%;
        background-color: rgba(255,255,255,0.6);
        z-index: 999;
    }
    .api-keys-container {
       padding: 0px 30px 55px 64px;
       overflow-y: scroll;
       max-height: 84vh;
       position: relative;

       &__content {
            display: grid;
            grid-template-columns: 190px 190px 190px 190px 190px 190px ;
            width: 100%;
            grid-row-gap: 20px;
            justify-content: space-between;
            margin-top: 175px;
            margin-bottom: 100px;
        }
   }

    @media screen and (max-width: 1600px) {
        .api-keys-container {

            &__content {
                grid-template-columns: 190px 190px 190px 190px 190px;
            }
        }
        .api-keys-header {
            max-width: 73.7%;
        }
    }

    @media screen and (max-width: 1366px) {
        .api-keys-container {

            &__content {
                grid-template-columns: 190px 190px 190px 190px;
            }
        }

        .api-keys-header {
            max-width: 72%;
        }
    }

    @media screen and (max-width: 1120px) {
        .api-keys-container {

            &__content {
                grid-template-columns: 190px 190px 190px;
                grid-row-gap: 0px;
            }
        }
        .api-keys-header {
            max-width: 82.7%;
        }
    }
</style>