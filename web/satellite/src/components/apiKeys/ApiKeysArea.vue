// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="api-keys-area">
        <h1>API Keys</h1>
        <div class="api-keys-area__container">
            <ApiKeysCreationPopup
                @closePopup="closeNewApiKeyPopup"
                @showCopyPopup="showCopyApiKeyPopup"
                :is-popup-shown="isNewApiKeyPopupShown"/>
            <ApiKeysCopyPopup
                :is-popup-shown="isCopyApiKeyPopupShown"
                :api-key-secret="apiKeySecret"
                @closePopup="closeCopyNewApiKeyPopup"/>
            <div v-if="!isEmpty" class="api-keys-header">
                <HeaderComponent ref="headerComponent" place-holder="API Key">
                    <div class="header-default-state" v-if="headerState === 0">
                        <Button class="button" label="+Create API Key" width="180px" height="48px" :on-press="onCreateApiKeyClick"/>
                    </div>
                    <div class="header-selected-api-keys" v-if="headerState === 1 && !isDeleteClicked">
                        <Button class="button deletion" label="Delete" width="122px" height="48px" :on-press="onFirstDeleteClick"/>
                        <Button class="button" label="Cancel" width="122px" height="48px" is-white="true" :on-press="onClearSelection"/>
                    </div>
                    <div class="header-after-delete-click" v-if="headerState === 1 && isDeleteClicked">
                        <span>Are you sure you want to delete {{selectedAPIKeysCount}} {{apiKeyCountTitle}} ?</span>
                        <div class="header-after-delete-click__button-area">
                            <Button class="button deletion" label="Delete" width="122px" height="48px" :on-press="onDelete"/>
                            <Button class="button" label="Cancel" width="122px" height="48px" is-white="true" :on-press="onClearSelection"/>
                        </div>
                    </div>
                </HeaderComponent>
                <div class="blur-content" v-if="isDeleteClicked"></div>
                <div class="blur-search" v-if="isDeleteClicked"></div>
            </div>
            <div v-if="!isEmpty" class="api-keys-items">
                <SortingHeader/>
                <div class="api-keys-items__content">
                    <List
                        :data-set="apiKeyList"
                        :item-component="itemComponent"
                        :on-item-click="toggleSelection"/>
                </div>
                <p>Want to give limited access? <b>Use API Keys.</b></p>
            </div>
            <EmptyState
                :on-button-click="onCreateApiKeyClick"
                v-if="isEmpty && !isNewApiKeyPopupShown"
                main-title="Let's create your first API Key"
                additional-text="<p>API keys give access to the project allowing you to create buckets, upload files, and read them. Once you’ve created an API key, you’re ready to interact with the network through our Uplink CLI.</p>"
                :image-source="emptyImage"
                button-label="Create an API Key"
                is-button-shown="true" />
        </div>
    </div>
</template>

<script lang="ts">
import VueClipboards from 'vue-clipboards';
import { Component, Vue } from 'vue-property-decorator';

import ApiKeysItem from '@/components/apiKeys/ApiKeysItem.vue';
import SortingHeader from '@/components/apiKeys/SortingHeader.vue';
import Button from '@/components/common/Button.vue';
import EmptyState from '@/components/common/EmptyStateArea.vue';
import HeaderComponent from '@/components/common/HeaderComponent.vue';
import List from '@/components/common/List.vue';

import { ApiKey } from '@/types/apiKeys';
import { API_KEYS_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

import ApiKeysCopyPopup from './ApiKeysCopyPopup.vue';
import ApiKeysCreationPopup from './ApiKeysCreationPopup.vue';

Vue.use(VueClipboards);

// header state depends on api key selection state
enum HeaderState {
    DEFAULT = 0,
    ON_SELECT,
}

const {
    FETCH,
    DELETE,
    TOGGLE_SELECTION,
    CLEAR_SELECTION,
} = API_KEYS_ACTIONS;

@Component({
    components: {
        List,
        EmptyState,
        HeaderComponent,
        ApiKeysItem,
        Button,
        ApiKeysCreationPopup,
        ApiKeysCopyPopup,
        SortingHeader,
    },
})
export default class ApiKeysArea extends Vue {
    public emptyImage: string = EMPTY_STATE_IMAGES.API_KEY;
    private isDeleteClicked: boolean = false;
    private isNewApiKeyPopupShown: boolean = false;
    private isCopyApiKeyPopupShown: boolean = false;
    private apiKeySecret: string = '';

    public mounted(): void {
        this.$store.dispatch(FETCH);
    }

    public toggleSelection(apiKey: ApiKey): void {
        this.$store.dispatch(TOGGLE_SELECTION, apiKey.id);
    }

    public onCreateApiKeyClick(): void {
        this.isNewApiKeyPopupShown = true;
    }

    public onFirstDeleteClick(): void {
        this.isDeleteClicked = true;
    }

    public onClearSelection(): void {
        this.$store.dispatch(CLEAR_SELECTION);
        this.isDeleteClicked = false;
    }

    public closeNewApiKeyPopup() {
        this.isNewApiKeyPopupShown = false;
    }

    public showCopyApiKeyPopup(secret: string) {
        this.isCopyApiKeyPopupShown = true;
        this.apiKeySecret = secret;
    }

    public closeCopyNewApiKeyPopup() {
        this.isCopyApiKeyPopupShown = false;
    }

    public async onDelete(): Promise<void> {
        const selectedKeys: string[] = this.$store.getters.selectedAPIKeys.map((key) => key.id);
        const keySuffix = selectedKeys.length > 1 ? '\'s' : '';

        try {
            await this.$store.dispatch(DELETE, selectedKeys);
            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, `API key${keySuffix} deleted successfully`);
        } catch (error) {
            this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, error.message);
        }

        this.isDeleteClicked = false;
    }

    public get itemComponent() {
        return ApiKeysItem;
    }

    public get apiKeyList(): ApiKey[] {
        return this.$store.getters.apiKeys;
    }

    public get apiKeyCountTitle(): string {
        if (this.selectedAPIKeysCount === 1) {
            return 'api key';
        }

        return 'api keys';
    }

    public get isEmpty(): boolean {
        return this.$store.getters.apiKeys.length === 0;
    }

    public get isSelected(): boolean {
        return this.$store.getters.selectedAPIKeys.length > 0;
    }

    public get selectedAPIKeysCount(): number {
        return this.$store.getters.selectedAPIKeys.length;
    }

    public get headerState(): number {
        if (this.selectedAPIKeysCount > 0) {
            return HeaderState.ON_SELECT;
        }

        return HeaderState.DEFAULT;
    }
}
</script>

<style scoped lang="scss">
    .api-keys-area {
        position: relative;
        padding: 40px 65px 55px 64px;
        height: 85vh;

        h1 {
            font-family: 'font_bold';
            font-size: 32px;
            line-height: 39px;
            margin: 0;
        }

        .api-keys-header {
            width: 100%;
            position: relative;

            .blur-content {
                position: absolute;
                top: 100%;
                left: 0;
                background-color: #F5F6FA;
                width: 100%;
                height: 70vh;
                z-index: 100;
                opacity: 0.3;
            }

            .blur-search {
                position: absolute;
                bottom: 0;
                right: 0;
                width: 602px;
                height: 56px;
                z-index: 100;
                opacity: 0.3;
                background-color: #F5F6FA;
            }
        }

        .api-keys-items {
            position: relative;

            &__content {
                display: flex;
                flex-direction: column;
                width: 100%;
                justify-content: flex-start;
                overflow-y: scroll;
                overflow-x: hidden;
                height: 49.4vh;
            }

            p {
                font-family: 'font_regular';
                font-size: 16px;
                color: #AFB7C1;
            }
        }
    }

    .header-default-state,
    .header-selected-api-keys {
        display: flex;
        align-items: center;
        position: relative;

        .button {
            position: absolute;
            top: -6px;
        }
    }

    .header-selected-api-keys {

        .button {
            position: absolute;
            top: -7px;
            left: 134px;
        }

        .deletion {
            position: absolute;
            top: -6px;
            left: 0
        }
    }

    .header-after-delete-click {
        display: flex;
        flex-direction: column;
        margin-top: 2px;

        span {
            font-family: 'font_medium';
            font-size: 14px;
            line-height: 28px;
        }

        &__button-area {
            display: flex;
            margin-top: 4px;

            .button {
                margin-top: 2px;
            }

            .deletion {
                margin: 3px 12px 0 0;
            }
        }
    }

    .container.deletion {
        background-color: #FF4F4D;

        &.label {
            color: #FFFFFF;
        }

        &:hover {
            background-color: #DE3E3D;
            box-shadow: none;
        }
    }

    h2 {
        font-family: 'font_bold';
        font-size: 24px;
        line-height: 29px;
        margin-bottom: 26px;
    }

    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        width: 0;
    }

    @media screen and (max-height: 800px) {
        .api-keys-items {

            &__content {
                height: 41.5vh !important;
            }
        }
    }
</style>
