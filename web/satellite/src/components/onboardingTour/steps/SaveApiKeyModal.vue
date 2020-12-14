// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="save-api-modal">
        <div class="save-api-modal__container">
            <OrangeExclamation/>
            <h1 class="save-api-modal__container__title">Is Your API Key Saved?</h1>
            <p class="save-api-modal__container__message">
                API Keys are only displayed once when generated. If you haven’t saved your key, go back to copy and
                paste the API key to your preferred method of storing secrets (i.e. TextEdit, Keybase, etc.)
            </p>
            <div class="save-api-modal__container__buttons-area">
                <VButton
                    class="back-button"
                    width="186px"
                    height="45px"
                    label="Go Back"
                    :on-press="onBackClick"
                    :is-blue-white="true"
                />
                <VButton
                    width="186px"
                    height="45px"
                    label="Yes, it's Saved!"
                    :on-press="onConfirmClick"
                />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import OrangeExclamation from '@/../static/images/onboardingTour/orange-exclamation.svg';

import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        OrangeExclamation,
        VButton,
    },
})

export default class SaveApiKeyModal extends Vue {
    /**
     * Toggles modal visibility.
     */
    public onBackClick(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SAVE_API_KEY_MODAL);
    }

    /**
     * Proceeds to tour's last step.
     */
    public onConfirmClick(): void {
        this.$emit('confirmSave');
    }
}
</script>

<style scoped lang="scss">
    .save-api-modal {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(9, 21, 35, 0.85);
        display: flex;
        align-items: center;
        justify-content: center;
        font-family: 'font_regular', sans-serif;

        &__container {
            background-color: #fff;
            z-index: 1;
            padding: 35px;
            display: flex;
            flex-direction: column;
            align-items: center;
            border-radius: 6px;
            max-width: 460px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 27px;
                color: #000;
                margin: 10px 0;
            }

            &__message {
                font-size: 16px;
                line-height: 24px;
                color: #000;
                word-break: break-word;
                text-align: center;
                margin: 0 0 10px 0;
            }

            &__buttons-area {
                display: flex;
                align-items: center;
            }
        }
    }

    .back-button {
        margin-right: 10px;
    }
</style>
