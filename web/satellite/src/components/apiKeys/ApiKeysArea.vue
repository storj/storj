// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="api-keys-area">
        <h1>Api Key</h1>
        <div class="api-keys-area__container">
            <ApiKeysCreationPopup
                @closePopup="closeNewApiKeyPopup"
                @showCopyPopup="showCopyApiKeyPopup"
                :isPopupShown="isNewApiKeyPopupShown"/>
            <ApiKeysCopyPopup
                :isPopupShown="isCopyApiKeyPopupShown"
                :apiKeySecret="apiKeySecret"
                @closePopup="closeCopyNewApiKeyPopup"/>
            <div v-if="!isEmpty" class="api-keys-header">
                <HeaderComponent ref="headerComponent" placeHolder="API Key">
                    <div class="header-default-state" v-if="headerState === 0">
                        <Button class="button" label="+Create API Key" width="180px" height="48px" :onPress="onCreateApiKeyClick"/>
                    </div>
                    <div class="header-selected-api-keys" v-if="headerState === 1 && !isDeleteClicked">
                        <Button class="button deletion" label="Delete" width="122px" height="48px" :onPress="onFirstDeleteClick"/>
                        <Button class="button" label="Cancel" width="122px" height="48px" isWhite="true" :onPress="onClearSelection"/>
                    </div>
                    <div class="header-after-delete-click" v-if="headerState === 1 && isDeleteClicked">
                        <span>Are you sure you want to delete {{selectedAPIKeysCount}} {{apiKeyCountTitle}}</span>
                        <div class="header-after-delete-click__button-area">
                            <Button class="button deletion" label="Delete" width="122px" height="48px" :onPress="onDelete"/>
                            <Button class="button" label="Cancel" width="122px" height="48px" isWhite="true" :onPress="onClearSelection"/>
                        </div>
                    </div>
                </HeaderComponent>
            </div>
            <div v-if="!isEmpty" class="api-keys-items">
                <div class="api-keys-items__content">
                    <SortingHeader/>
                    <List
                        :dataSet="apiKeyList"
                        :itemComponent="itemComponent"
                        :onItemClick="toggleSelection"/>
                </div>
            </div>
            <EmptyState
                :onButtonClick="onCreateApiKeyClick"
                v-if="isEmpty && !isNewApiKeyPopupShown"
                mainTitle="Let's create your first API Key"
                additional-text="<p>API keys give access to the project allowing you to create buckets, upload files, and read them. Once you’ve created an API key, you’re ready to interact with the network through our Uplink CLI.</p>"
                :imageSource="emptyImage"
                buttonLabel="Create an API Key"
                isButtonShown="true" />
        </div>
    </div>
</template>

<script lang="ts">    import { Component, Vue } from 'vue-property-decorator';
    import VueClipboards from 'vue-clipboards';
    import ApiKeysCreationPopup from './ApiKeysCreationPopup.vue';
    import ApiKeysCopyPopup from './ApiKeysCopyPopup.vue';
    import ApiKeysItem from '@/components/apiKeys/ApiKeysItem.vue';
    import Button from '@/components/common/Button.vue';
    import EmptyState from '@/components/common/EmptyStateArea.vue';
    import List from "@/components/common/List.vue";
    import HeaderComponent from '@/components/common/HeaderComponent.vue';
    import SortingHeader from "@/components/apiKeys/SortingHeader.vue";
    import { API_KEYS_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
    import { ApiKey } from '@/types/apiKeys';
    import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
    import { RequestResponse } from '@/types/response';

    Vue.use(VueClipboards);

    // header state depends on api key selection state
    enum HeaderState {
        DEFAULT = 0,
        ON_SELECT,
    }

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
            this.$store.dispatch(API_KEYS_ACTIONS.FETCH);
        }

        public toggleSelection(apiKey: ApiKey): void {
            this.$store.dispatch(API_KEYS_ACTIONS.TOGGLE_SELECTION, apiKey.id);
        }

        public onCreateApiKeyClick(): void {
            this.isNewApiKeyPopupShown = true;
        }

        public onFirstDeleteClick(): void {
            this.isDeleteClicked = true;
        }

        public onClearSelection(): void {
            this.$store.dispatch(API_KEYS_ACTIONS.CLEAR_SELECTION);
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
            let selectedKeys: string[] = this.$store.getters.selectedAPIKeys.map((key) => { return key.id; });

            const dispatchResult: RequestResponse<null> = await this.$store.dispatch(API_KEYS_ACTIONS.DELETE, selectedKeys);

            let keySuffix = selectedKeys.length > 1 ? '\'s' : '';

            if (dispatchResult.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, `API key${keySuffix} deleted successfully`);
            } else {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, dispatchResult.errorMessage);
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
        max-width: 92.9%;
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
        }

        .api-keys-items {
            position: relative;
            overflow-y: scroll;
            overflow-x: hidden;
            height: 82vh;

            &__content {
                display: flex;
                flex-direction: column;
                width: 100%;
                justify-content: flex-start;
                margin-top: 20px;
                margin-bottom: 100px;
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

    /*/deep/ .apikey-item-container.selected {*/
    /*    background-color: red;*/
    /*}*/

    @media screen and (max-width: 1840px) {
        .api-keys-area {

            .api-keys-items__content {
                grid-template-columns: 200px 200px 200px 200px 200px 200px;
            }
        }
    }

    @media screen and (max-width: 1695px) {
        .api-keys-area {

            .api-keys-items__content  {
                grid-template-columns: 180px 180px 180px 180px 180px 180px;
            }
        }

        .apikey-item-container {
            height: 180px;
        }
    }

    @media screen and (max-width: 1575px) {
        .api-keys-area {

            .api-keys-items__content  {
                grid-template-columns: 200px 200px 200px 200px 200px;
            }
        }

        .apikey-item-container {
            height: 200px;
        }
    }

    @media screen and (max-width: 1475px) {
        .api-keys-area {

            .api-keys-items__content  {
                grid-template-columns: 180px 180px 180px 180px 180px;
            }
        }

        .apikey-item-container {
            height: 180px;
        }
    }

    @media screen and (max-width: 1375px) {
        .api-keys-area {

            .api-keys-items__content  {
                grid-template-columns: 200px 200px 200px 200px;
            }
        }

        .apikey-item-container {
            height: 200px;
        }
    }

    @media screen and (max-width: 1250px) {
        .api-keys-area {

            .api-keys-items__content  {
                grid-template-columns: 180px 180px 180px 180px;
            }
        }

        .apikey-item-container {
            height: 180px;
        }
    }

    @media screen and (max-width: 1160px) {
        .api-keys-area {

            .api-keys-items__content  {
                grid-template-columns: 205px 205px 205px;
            }
        }

        .apikey-item-container {
            height: 205px;
        }
    }

    @media screen and (max-width: 840px) {
        .api-keys-area {

            .api-keys-items__content  {
                grid-template-columns: 180px 180px 180px;
            }
        }

        .apikey-item-container {
            height: 180px;
        }
    }
</style>
