// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="prompt">
        <div class="prompt__modal">
            <img
                class="prompt__modal__icon"
                src="@/../static/images/account/billing/paidTier/prompt.png"
                alt="Prompt Image"
            >
            <h1 class="prompt__modal__title" aria-roledescription="modal-title">
                Get more projects<br>when you upgrade
            </h1>
            <p class="prompt__modal__info">
                Upgrade your Free Account to create<br>more projects and gain access to higher limits.
            </p>
            <VButton
                width="256px"
                height="56px"
                border-radius="8px"
                label="Upgrade to Pro Account ->"
                :on-press="onClick"
            />
            <div class="close-cross-container" @click="onClose">
                <CloseCrossIcon />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { APP_STATE_MUTATIONS } from "@/store/mutationConstants";
import { PAYMENTS_MUTATIONS } from "@/store/modules/payments";
import VButton from '@/components/common/VButton.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

// @vue/component
@Component({
    components: {
        VButton,
        CloseCrossIcon,
    },
})
export default class CreateProjectPromptModal extends Vue {
    @Prop({default: () => false})
    public readonly onClose: () => void;

    /**
     * Holds on button click logic.
     * Closes this modal and opens upgrade account modal.
     */
    public onClick(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_CREATE_PROJECT_PROMPT_POPUP);
        this.$store.commit(PAYMENTS_MUTATIONS.TOGGLE_IS_ADD_PM_MODAL_SHOWN);
    }
}
</script>

<style scoped lang="scss">
    .prompt {
        position: fixed;
        top: 0;
        right: 0;
        left: 0;
        bottom: 0;
        z-index: 1000;
        background: rgb(27 37 51 / 75%);
        display: flex;
        align-items: center;
        justify-content: center;
        font-family: 'font_regular', sans-serif;

        &__modal {
            display: flex;
            align-items: center;
            flex-direction: column;
            background: #fff;
            border-radius: 20px;
            box-shadow: 0 0 32px rgb(0 0 0 / 4%);
            width: 600px;
            position: relative;
            padding: 50px 0 65px;

            &__icon {
                max-height: 154px;
                max-width: 118px;
            }

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 28px;
                line-height: 34px;
                color: #1b2533;
                margin-top: 40px;
                text-align: center;
            }

            &__info {
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                line-height: 21px;
                text-align: center;
                color: #354049;
                margin: 15px 0 45px;
            }
        }
    }

    .close-cross-container {
        display: flex;
        justify-content: center;
        align-items: center;
        position: absolute;
        right: 30px;
        top: 30px;
        height: 24px;
        width: 24px;
        cursor: pointer;

        &:hover .close-cross-svg-path {
            fill: #2683ff;
        }
    }

    @media screen and (max-height: 900px) {

        .prompt {
            padding: 150px 0 20px;
            overflow-y: scroll;
        }
    }
</style>
