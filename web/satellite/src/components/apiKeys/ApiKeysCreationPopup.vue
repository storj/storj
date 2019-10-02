// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="new-api-key" v-if="isPopupShown">
        <h2 class="new-api-key__title">Name Your API Key</h2>
        <HeaderlessInput
            @setData="onChangeName"
            :error="errorMessage"
            placeholder="Enter API Key Name"
            class="full-input"
            width="100%"
        />
        <VButton
            class="next-button"
            label="Next >"
            width="128px"
            height="48px"
            :on-press="onNextClick"
        />
        <div class="new-api-key__close-cross-container" @click="onCloseClick">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path class="close-cross-svg-path" d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
            </svg>
        </div>
        <div class="blur-content"></div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import VButton from '@/components/common/VButton.vue';

import { ApiKey } from '@/types/apiKeys';
import { API_KEYS_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

const CREATE = API_KEYS_ACTIONS.CREATE;

@Component({
    components: {
        HeaderlessInput,
        VButton,
    },
})
export default class ApiKeysCreationPopup extends Vue {
    @Prop({default: false})
    private readonly isPopupShown: boolean;

    private name: string = '';
    private errorMessage: string = '';
    private isLoading: boolean = false;
    private key: string = '';

    private FIRST_PAGE = 1;

    public onChangeName(value: string): void {
        this.name = value.trim();
        this.errorMessage = '';
    }

    public onCloseClick(): void {
        this.onChangeName('');
        this.$emit('closePopup');
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

        let createdApiKey: ApiKey;

        try {
            createdApiKey = await this.$store.dispatch(CREATE, this.name);
        } catch (error) {
            this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, error.message);
            this.isLoading = false;

            return;
        }

        this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Successfully created new api key');
        this.key = createdApiKey.secret;
        this.isLoading = false;

        try {
            this.$store.dispatch(API_KEYS_ACTIONS.FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, `Unable to fetch API keys. ${error.message}`);
        }

        this.$emit('closePopup');
        this.$emit('showCopyPopup', this.key);
    }
}
</script>

<style scoped lang="scss">
    .new-api-key {
        padding: 32px 58px 41px 40px;
        background-color: #FFFFFF;
        border-radius: 24px;
        margin-top: 29px;
        max-width: 93.5%;
        height: auto;
        position: relative;

        &__title {
            font-family: 'font_bold';
            font-size: 24px;
            line-height: 29px;
            margin-bottom: 26px;
        }

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

            &:hover .close-cross-svg-path {
                fill: #2683FF;
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
    }
</style>
