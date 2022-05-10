// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="mfa-codes">
        <div class="mfa-codes__container">
            <h1 class="mfa-codes__container__title">Two-Factor Authentication</h1>
            <div class="mfa-codes__container__codes">
                <p class="mfa-codes__container__codes__subtitle">
                    Please save these codes somewhere to be able to recover access to your account.
                </p>
                <p
                    v-for="(code, index) in userMFARecoveryCodes"
                    :key="index"
                >
                    {{ code }}
                </p>
            </div>
            <VButton
                class="done-button"
                label="Done"
                width="100%"
                height="44px"
                :on-press="toggleModal"
            />
            <div class="mfa-codes__container__close-container" @click="toggleModal">
                <CloseCrossIcon />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

// @vue/component
@Component({
    components: {
        CloseCrossIcon,
        VButton,
    },
})
export default class MFARecoveryCodesPopup extends Vue {
    @Prop({default: () => () => false})
    public readonly toggleModal: () => void;

    /**
     * Returns MFA recovery codes from store.
     */
    public get userMFARecoveryCodes(): string[] {
        return this.$store.state.usersModule.userMFARecoveryCodes;
    }
}
</script>

<style scoped lang="scss">
    .mfa-codes {
        position: fixed;
        top: 0;
        bottom: 0;
        right: 0;
        left: 0;
        display: flex;
        justify-content: center;
        z-index: 1000;
        background: rgb(27 37 51 / 75%);

        &__container {
            padding: 60px;
            height: fit-content;
            margin-top: 100px;
            position: relative;
            background: #fff;
            border-radius: 6px;
            display: flex;
            flex-direction: column;
            align-items: center;
            font-family: 'font_regular', sans-serif;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 28px;
                line-height: 34px;
                text-align: center;
                color: #000;
                margin: 0 0 30px;
            }

            &__codes {
                padding: 25px;
                background: #f5f6fa;
                border-radius: 6px;
                width: calc(100% - 50px);
                display: flex;
                flex-direction: column;
                align-items: center;

                &__subtitle {
                    font-size: 16px;
                    line-height: 21px;
                    text-align: center;
                    color: #000;
                    margin: 0 0 30px;
                    max-width: 485px;
                }
            }

            &__close-container {
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
        }
    }

    .done-button {
        margin-top: 30px;
    }

    @media screen and (max-height: 750px) {

        .mfa-codes {
            padding-bottom: 20px;
            overflow-y: scroll;
        }
    }
</style>
