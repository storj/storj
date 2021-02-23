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
            <CloseCrossIcon/>
        </div>
        <div class="blur-content"></div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import VButton from '@/components/common/VButton.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

import { API_KEYS_ACTIONS } from '@/store/modules/apiKeys';
import { ApiKey } from '@/types/apiKeys';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

const {
    CREATE,
    FETCH,
} = API_KEYS_ACTIONS;

@Component({
    components: {
        HeaderlessInput,
        VButton,
        CloseCrossIcon,
    },
})
export default class ApiKeysCreationPopup extends Vue {
    /**
     * Indicates if component should be rendered.
     */
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

    /**
     * Creates api key.
     */
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
            await this.$notify.error(error.message);
            this.isLoading = false;

            return;
        }

        await this.$notify.success('Successfully created new api key');
        this.key = createdApiKey.secret;
        this.isLoading = false;
        this.name = '';

        this.$segment.track(SegmentEvent.API_KEY_CREATED, {
            project_id: this.$store.getters.selectedProject.id,
        });

        try {
            await this.$store.dispatch(FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch API keys. ${error.message}`);
        }

        this.$emit('closePopup');
        this.$emit('showCopyPopup', this.key);
    }
}
</script>

<style scoped lang="scss">
    .new-api-key {
        padding: 32px 58px 41px 40px;
        background-color: #fff;
        border-radius: 24px;
        margin-top: 29px;
        max-width: 93.5%;
        height: auto;
        position: relative;

        &__title {
            font-family: 'font_bold', sans-serif;
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
                fill: #2683ff;
            }
        }

        .blur-content {
            position: absolute;
            top: 100%;
            left: 0;
            background-color: #f5f6fa;
            width: 100%;
            height: 70.5vh;
            z-index: 100;
            opacity: 0.3;
        }
    }
</style>
