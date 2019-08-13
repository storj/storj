// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="api-keys-area">
        <h1>Api Key</h1>
        <div class="api-keys-area__container">
            <div class="api-keys-area__container__new-api-key" v-if="isNewApiKeyPopupShown">
                <h2>Name Your API Key</h2>
                <HeaderlessInput
                    @setData="onChangeName"
                    :error="errorMessage"
                    placeholder="Enter API Key Name"
                    class="full-input"
                    width="100%" />
                <Button class="next-button" label="Next >" width="128px" height="48px" :onPress="onNextClick" />
                <div class="api-keys-area__container__new-api-key__close-cross-container" @click="onCloseClick">
                    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                    </svg>
                </div>
                <div class="blur-content"></div>
            </div>
            <div class="api-keys-area__container__save-api-popup" v-if="isCopyApiKeyPopupShown">
                <h2>Name Your API Key</h2>
                <div class="api-keys-area__container__save-api-popup__copy-area">
                    <div class="api-keys-area__container__save-api-popup__copy-area__key-area">
                        <p>{{key}}</p>
                    </div>
                    <div class="copy-button" v-clipboard="key" @click="onCopyClick" v-if="!isCopiedButtonShown">
                        <svg width="22" height="22" viewBox="0 0 22 22" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M13.3763 7.7002H3.34383C2.46383 7.7002 1.76001 8.40402 1.76001 9.28402V19.3378C1.76001 20.1954 2.46383 20.9216 3.34383 20.9216H13.3976C14.2553 20.9216 14.9814 20.2178 14.9814 19.3378L14.9823 9.28402C14.96 8.40402 14.2561 7.7002 13.3761 7.7002H13.3763ZM13.6401 19.3164C13.6401 19.4488 13.5301 19.5588 13.3977 19.5588L3.34397 19.5579C3.21162 19.5579 3.10161 19.4479 3.10161 19.3156L3.10247 9.284C3.10247 9.15165 3.21247 9.04164 3.34483 9.04164H13.3986C13.5309 9.04164 13.641 9.15164 13.641 9.284L13.6401 19.3164Z" fill="white"/>
                            <path d="M18.6563 1.09974H8.62386C7.74386 1.09974 7.04004 1.80356 7.04004 2.68356V6.37978H8.36004V2.68356C8.36004 2.55122 8.47004 2.44121 8.60239 2.44121H18.6562C18.7885 2.44121 18.8985 2.55121 18.8985 2.68356V12.7373C18.8985 12.8697 18.7885 12.9797 18.6562 12.9797H16.2799V14.2997H18.6562C19.5138 14.2997 20.24 13.5959 20.24 12.7159V2.68343C20.24 1.80343 19.5362 1.09961 18.6562 1.09961L18.6563 1.09974Z" fill="#354049"/>
                            <rect x="2.93335" y="8.7998" width="11.7333" height="11" fill="white"/>
                            <rect x="7.1001" y="1.2334" width="12.9333" height="12.9333" rx="1.5" fill="white" stroke="#2683FF"/>
                        </svg>
                        <p>Copy</p>
                    </div>
                    <div class="copied-button" v-if="isCopiedButtonShown">
                        <svg width="22" height="22" viewBox="0 0 22 22" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M13.3763 7.7002H3.34383C2.46383 7.7002 1.76001 8.40402 1.76001 9.28402V19.3378C1.76001 20.1954 2.46383 20.9216 3.34383 20.9216H13.3976C14.2553 20.9216 14.9814 20.2178 14.9814 19.3378L14.9823 9.28402C14.96 8.40402 14.2561 7.7002 13.3761 7.7002H13.3763ZM13.6401 19.3164C13.6401 19.4488 13.5301 19.5588 13.3977 19.5588L3.34397 19.5579C3.21162 19.5579 3.10161 19.4479 3.10161 19.3156L3.10247 9.284C3.10247 9.15165 3.21247 9.04164 3.34483 9.04164H13.3986C13.5309 9.04164 13.641 9.15164 13.641 9.284L13.6401 19.3164Z" fill="white"/>
                            <path d="M18.6563 1.09974H8.62386C7.74386 1.09974 7.04004 1.80356 7.04004 2.68356V6.37978H8.36004V2.68356C8.36004 2.55122 8.47004 2.44121 8.60239 2.44121H18.6562C18.7885 2.44121 18.8985 2.55121 18.8985 2.68356V12.7373C18.8985 12.8697 18.7885 12.9797 18.6562 12.9797H16.2799V14.2997H18.6562C19.5138 14.2997 20.24 13.5959 20.24 12.7159V2.68343C20.24 1.80343 19.5362 1.09961 18.6562 1.09961L18.6563 1.09974Z" fill="#354049"/>
                            <rect x="2.93335" y="8.7998" width="11.7333" height="11" fill="white"/>
                            <rect x="7.1001" y="1.2334" width="12.9333" height="12.9333" rx="1.5" fill="white" stroke="#2683FF"/>
                        </svg>
                        <p>Copied</p>
                    </div>
                </div>
                <div class="api-keys-area__container__new-api-key__close-cross-container" @click="onCloseClick">
                    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                    </svg>
                </div>
                <div class="blur-content"></div>
            </div>
            <div class="api-keys-area__container__content">
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
                        <div v-for="apiKey in apiKeyList" v-on:click="toggleSelection(apiKey.id)">
                            <ApiKeysItem
                                v-bind:class="[apiKey.isSelected ? 'selected': null]"
                                :apiKey="apiKey" />
                        </div>
                    </div>
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

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import EmptyState from '@/components/common/EmptyStateArea.vue';
    import HeaderComponent from '@/components/common/HeaderComponent.vue';
    import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
    import ApiKeysItem from '@/components/apiKeys/ApiKeysItem.vue';
    import { API_KEYS_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
    import { ApiKey } from '@/types/apiKeys';
    import Button from '@/components/common/Button.vue';
    import { RequestResponse } from '@/types/response';
    import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
    import VueClipboards from 'vue-clipboards';

    Vue.use(VueClipboards);

    // header state depends on api key selection state
    enum HeaderState {
        DEFAULT = 0,
        ON_SELECT,
    }

    @Component({
        components: {
            EmptyState,
            HeaderComponent,
            ApiKeysItem,
            Button,
            HeaderlessInput
        },
    })
    export default class ApiKeysArea extends Vue {
        public emptyImage: string = EMPTY_STATE_IMAGES.API_KEY;
        private name: string = '';
        private errorMessage: string = '';
        private key: string = '';
        private isLoading: boolean = false;
        private isNewApiKeyPopupShown: boolean = false;
        private isCopyApiKeyPopupShown: boolean = false;
        private isCopiedButtonShown: boolean = false;
        private isDeleteClicked: boolean = false;

        public mounted(): void {
            this.$store.dispatch(API_KEYS_ACTIONS.FETCH);
        }

        public toggleSelection(id: string): void {
            this.$store.dispatch(API_KEYS_ACTIONS.TOGGLE_SELECTION, id);
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

        public onCloseClick(): void {
            this.isNewApiKeyPopupShown = false;
            this.isCopyApiKeyPopupShown = false;
            this.isCopiedButtonShown = false;
            this.isDeleteClicked = false;
        }

        public onChangeName(value: string): void {
            this.name = value.trim();
            this.errorMessage = '';
        }

        public onCopyClick(): void {
            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Key saved to clipboard');
            this.isCopiedButtonShown = true;
        }

        public async onNextClick(): Promise<void> {
            if (this.isLoading) {
                return;
            }

            if (!this.name) {
                this.errorMessage = 'API Key name can`t be empty';

                return;
            }

            this.isLoading = true;

            let result: any = await this.$store.dispatch(API_KEYS_ACTIONS.CREATE, this.name);
            if (!result.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, result.errorMessage);
                this.isLoading = false;

                return;
            }

            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Successfully created new api key');
            this.key = result.data.secret;

            this.isLoading = false;
            this.isNewApiKeyPopupShown = false;
            this.isCopyApiKeyPopupShown = true;
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

        public get apiKeyList(): ApiKey[] {
            return this.$store.state.apiKeysModule.apiKeys;
        }

        public get apiKeyCountTitle(): string {
            if (this.selectedAPIKeysCount === 1) {
                return 'api key';
            }

            return 'api keys';
        }

        public get isEmpty(): boolean {
            return this.$store.state.apiKeysModule.apiKeys.length === 0;
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

        &__container {

            &__new-api-key {
                padding: 32px 58px 41px 40px;
                background-color: #FFFFFF;
                border-radius: 24px;
                margin-top: 29px;
                max-width: 93.5%;
                height: auto;
                position: relative;

                .next-button {
                    margin-top: 20px;
                }

                &__close-cross-container {
                    display: flex;
                    justify-content: center;
                    align-items: center;
                    position: absolute;
                    right: 29px;
                    top: 29px;
                    height: 24px;
                    width: 24px;
                    cursor: pointer;

                    &:hover svg path {
                        fill: #2683FF;
                    }
                }
            }

            &__save-api-popup {
                padding: 32px 40px 60px 40px;
                background-color: #FFFFFF;
                border-radius: 24px;
                margin-top: 29px;
                max-width: 94.8%;
                height: auto;
                position: relative;

                &__copy-area {
                    display: flex;
                    align-items: center;
                    justify-content: space-between;
                    background-color: #F5F6FA;
                    padding: 29px 32px 29px 24px;
                    border-radius: 12px;
                    position: relative;

                    &__key-area {

                        p {
                            font-family: 'font_regular';
                            margin: 0;
                            font-size: 16px;
                            line-height: 21px;
                            word-break: break-all;
                        }
                    }

                    .copy-button,
                    .copied-button {
                        display: flex;
                        background-color: #2683FF;
                        padding: 13px 36px;
                        cursor: pointer;
                        align-items: center;
                        justify-content: space-between;
                        color: #FFFFFF;
                        border: 1px solid #2683FF;
                        box-sizing: border-box;
                        border-radius: 8px;
                        font-size: 14px;
                        font-family: 'font_bold';
                        margin-left: 10px;

                        p {
                            margin: 0;
                        }

                        &:hover {
                            background-color: #196CDA;
                        }
                    }

                    .copied-button {
                        padding: 13px 28.5px;
                        background-color: #196CDA;
                        cursor: default;
                    }
                }
            }
        }

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

        .api-keys-header {
            width: 100%;
        }

        .api-keys-items {
            position: relative;
            overflow-y: scroll;
            overflow-x: hidden;
            height: 82vh;

            &__content {
                display: grid;
                grid-template-columns: 190px 190px 190px 190px 190px 190px 190px;
                width: 100%;
                grid-row-gap: 20px;
                grid-column-gap: 20px;
                justify-content: space-between;
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
