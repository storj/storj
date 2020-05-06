// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-api-key-step">
        <h1 class="create-api-key-step__title">Create an API Key</h1>
        <p class="create-api-key-step__sub-title">
            API keys provide access to the project for creating buckets and uploading objects through the command line
            interface. This will be your first API key, and you can always create more keys later on.
        </p>
        <div class="create-api-key-step__container">
            <div class="create-api-key-step__container__title-area">
                <h2 class="create-api-key-step__container__title-area__title">Create API Key</h2>
                <img
                    v-if="isLoading"
                    class="create-api-key-step__container__title-area__loading-image"
                    src="@/../static/images/account/billing/loading.gif"
                    alt="loading gif"
                >
            </div>
            <HeaderedInput
                label="API Key Name"
                placeholder="Enter API Key Name (i.e. Dan’s Key)"
                class="full-input"
                width="calc(100% - 4px)"
                :error="errorMessage"
                @setData="setApiKeyName"
            />
            <div class="create-api-key-step__container__create-key-area" v-if="isCreatingState">
                <VButton
                    class="generate-button"
                    width="100%"
                    height="40px"
                    label="Generate API Key"
                    :is-blue-white="true"
                    :on-press="createApiKey"
                />
            </div>
            <div class="create-api-key-step__container__copy-key-area" v-else>
                <div class="create-api-key-step__container__copy-key-area__header">
                    <InfoImage/>
                    <span class="create-api-key-step__container__copy-key-area__header__title">
                        API Keys only appear here once. Copy and paste this key to your preferred method of storing secrets.
                    </span>
                </div>
                <div class="create-api-key-step__container__copy-key-area__key-container">
                    <span class="create-api-key-step__container__copy-key-area__key-container__key">{{ key }}</span>
                    <div class="create-api-key-step__container__copy-key-area__key-container__copy-button">
                        <VButton
                            width="81px"
                            height="40px"
                            label="Copy"
                            :is-blue-white="true"
                            :on-press="onCopyClick"
                        />
                    </div>
                </div>
            </div>
            <p class="create-api-key-step__container__info" v-if="isCopyState">
                We don’t record your API Keys, which are only displayed once when generated. If you loose this
                key, it cannot be recovered – but you can always create new API Keys when needed.
            </p>
            <div class="create-api-key-step__container__blur" v-if="isLoading"/>
        </div>
        <VButton
            class="done-button"
            width="156px"
            height="48px"
            label="Done"
            :on-press="onDoneClick"
            :is-disabled="isCreatingState"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';

import InfoImage from '@/../static/images/onboardingTour/info.svg';

import { ApiKey } from '@/types/apiKeys';
import { API_KEYS_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { AddingApiKeyState } from '@/utils/constants/onboardingTourEnums';

const {
    CREATE,
    FETCH,
} = API_KEYS_ACTIONS;

@Component({
    components: {
        VButton,
        HeaderedInput,
        InfoImage,
    },
})
export default class CreateApiKeyStep extends Vue {
    private name: string = '';
    private addingState: number = AddingApiKeyState.CREATE;
    private readonly FIRST_PAGE = 1;

    public key: string = '';
    public errorMessage: string = '';
    public isLoading: boolean = false;

    /**
     * Indicates if view is in creating state.
     */
    public get isCreatingState(): boolean {
        return this.addingState === AddingApiKeyState.CREATE;
    }

    /**
     * Indicates if view is in copy state.
     */
    public get isCopyState(): boolean {
        return this.addingState === AddingApiKeyState.COPY;
    }

    /**
     * Indicates view state to copy state.
     */
    public setCopyState(): void {
        this.addingState = AddingApiKeyState.COPY;
    }

    /**
     * Sets api key name from input value.
     */
    public setApiKeyName(value: string): void {
        this.name = value.trim();
        this.errorMessage = '';
    }

    /**
     * Creates api key and refreshes store.
     */
    public async createApiKey(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        if (!this.name) {
            this.errorMessage = 'API Key name can`t be empty';

            return;
        }

        this.isLoading = true;

        let createdApiKey: ApiKey;

        try {
            createdApiKey = await this.$store.dispatch(CREATE, this.name);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;

            return;
        }

        await this.$notify.success('Successfully created new api key');
        this.key = createdApiKey.secret;

        this.$segment.track(SegmentEvent.API_KEY_CREATED, {
            project_id: this.$store.getters.selectedProject.id,
        });

        try {
            await this.$store.dispatch(FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch API keys. ${error.message}`);
        }

        this.setCopyState();

        this.isLoading = false;
    }

    /**
     * Copies api key secret to buffer.
     */
    public onCopyClick(): void {
        this.$copyText(this.key);
        this.$notify.success('Key saved to clipboard');
    }

    /**
     * Sets tour state to last step.
     */
    public onDoneClick(): void {
        this.$emit('setUploadDataState');
    }
}
</script>

<style scoped lang="scss">
    h1,
    h2,
    p {
        margin: 0;
    }

    .create-api-key-step {
        font-family: 'font_regular', sans-serif;
        margin-top: 75px;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: space-between;
        padding: 0 200px;

        &__title {
            font-size: 32px;
            line-height: 39px;
            color: #1b2533;
            margin-bottom: 25px;
        }

        &__sub-title {
            font-size: 16px;
            line-height: 19px;
            color: #354049;
            margin-bottom: 35px;
            text-align: center;
            word-break: break-word;
        }

        &__container {
            padding: 50px;
            width: calc(100% - 100px);
            border-radius: 8px;
            background-color: #fff;
            position: relative;
            margin-bottom: 30px;

            &__title-area {
                display: flex;
                align-items: center;
                justify-content: flex-start;
                margin-bottom: 10px;

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-size: 22px;
                    line-height: 27px;
                    color: #354049;
                    margin-right: 15px;
                }

                &__loading-image {
                    width: 18px;
                    height: 18px;
                }
            }

            &__create-key-area {
                width: calc(100% - 90px);
                padding: 40px 45px;
                margin-top: 30px;
                background-color: #0c2546;
                border-radius: 8px;
            }

            &__copy-key-area {
                width: 100%;
                margin-top: 30px;

                &__header {
                    padding: 10px;
                    width: calc(100% - 20px);
                    background-color: #ce3030;
                    display: flex;
                    align-items: center;
                    justify-content: flex-start;
                    border-radius: 8px 8px 0 0;

                    &__title {
                        font-size: 12px;
                        line-height: 16px;
                        color: #fff;
                    }
                }

                &__key-container {
                    background-color: #0c2546;
                    display: flex;
                    align-items: center;
                    padding: 20px 25px;
                    width: calc(100% - 50px);
                    border-radius: 0 0 8px 8px;

                    &__key {
                        font-size: 15px;
                        line-height: 23px;
                        color: #fff;
                        margin-right: 50px;
                        word-break: break-all;
                    }

                    &__copy-button {
                        min-width: 85px;
                    }
                }
            }

            &__info {
                width: 100%;
                margin-top: 30px;
                font-size: 12px;
                line-height: 18px;
                text-align: center;
                color: #354049;
                word-break: break-word;
            }

            &__blur {
                position: absolute;
                top: 0;
                left: 0;
                height: 100%;
                width: 100%;
                background-color: rgba(229, 229, 229, 0.2);
                z-index: 100;
            }
        }
    }

    .full-input {
        width: 100%;
    }

    .info-svg {
        min-width: 18px;
        margin-right: 5px;
    }

    @media screen and (max-width: 1650px) {

        .create-api-key-step {
            padding: 0 100px;
        }
    }

    @media screen and (max-width: 1450px) {

        .create-api-key-step {
            padding: 0 60px;
        }
    }

    @media screen and (max-width: 900px) {

        .create-api-key-step {
            padding: 0 50px;
        }
    }
</style>
