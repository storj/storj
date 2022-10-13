// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <h1 class="modal__title" aria-roledescription="modal-title">
                    Add STORJ Tokens
                </h1>
                <p class="modal__info">
                    Send STORJ Tokens to the following deposit address to credit your Storj DCS account:
                </p>
                <VLoader v-if="isLoading" class="modal__loader" width="100px" height="100px" />
                <template v-else>
                    <div class="modal__qr">
                        <canvas ref="canvas" class="modal__qr__canvas" />
                    </div>
                    <div class="modal__label">
                        <h2 class="modal__label__text">Deposit Address</h2>
                        <VInfo class="modal__label__info">
                            <template #icon>
                                <InfoIcon />
                            </template>
                            <template #message>
                                <p class="modal__label__info__msg">
                                    This is a Storj deposit address generated just for you.
                                    <a
                                        class="modal__label__info__msg__link"
                                        href=""
                                        target="_blank"
                                        rel="noopener noreferrer"
                                    >
                                        Learn more
                                    </a>
                                </p>
                            </template>
                        </VInfo>
                    </div>
                    <div class="modal__address">
                        <p class="modal__address__value">{{ wallet.address }}</p>
                        <VButton
                            class="modal__address__copy-button"
                            label="Copy"
                            width="84px"
                            height="32px"
                            font-size="13px"
                            icon="copy"
                            :on-press="onCopyAddressClick"
                        />
                    </div>
                </template>
                <VButton
                    width="194px"
                    height="48px"
                    border-radius="8px"
                    label="Done"
                    font-size="14px"
                    :on-press="closeModal"
                />
                <div class="modal__footer">
                    <h2 class="modal__footer__title">Send only STORJ Tokens to this deposit address.</h2>
                    <p class="modal__footer__msg">
                        Sending coin or token other than STORJ Token may result in the loss of your deposit.
                    </p>
                </div>
            </div>
        </template>
    </VModal>
</template>

<script lang="ts">
import QRCode from 'qrcode';
import { Component, Vue } from 'vue-property-decorator';

import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { Wallet } from '@/types/payments';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';
import VInfo from '@/components/common/VInfo.vue';
import VLoader from '@/components/common/VLoader.vue';

import InfoIcon from '@/../static/images/payments/infoIcon.svg';

// @vue/component
@Component({
    components: {
        VButton,
        VModal,
        VInfo,
        VLoader,
        InfoIcon,
    },
})
export default class AddTokenFundsModal extends Vue {
    private isLoading = this.wallet.address === '';

    public $refs!: {
        canvas: HTMLCanvasElement;
    };

    /**
     * Mounted lifecycle hook after initial render.
     * Fetches wallet if necessary and renders QR code.
     */
    public async mounted(): Promise<void> {
        try {
            if (!this.wallet.address) {
                await this.$store.dispatch(PAYMENTS_ACTIONS.CLAIM_WALLET);
            }

            await QRCode.toCanvas(this.$refs.canvas, this.wallet.address);
            this.isLoading = false;
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Closes create project prompt modal.
     */
    public closeModal(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_ADD_TOKEN_FUNDS_MODAL_SHOWN);
    }

    /**
     * Copies address to user's clipboard.
     */
    public onCopyAddressClick(): void {
        this.$copyText(this.wallet.address);
        this.$notify.success('Address copied to your clipboard');
    }

    /**
     * Returns wallet from store.
     */
    private get wallet(): Wallet {
        return this.$store.state.paymentsModule.wallet;
    }
}
</script>

<style scoped lang="scss">
    .modal {
        width: 546px;
        padding: 48px 0 0;
        display: flex;
        align-items: center;
        flex-direction: column;
        font-family: 'font_regular', sans-serif;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
            letter-spacing: -0.02em;
            color: #1b2533;
        }

        &__info {
            font-family: 'font_regular', sans-serif;
            font-size: 16px;
            line-height: 24px;
            text-align: center;
            color: #000;
            margin: 15px 0;
            max-width: 410px;
        }

        &__loader {
            margin-bottom: 20px;
        }

        &__label {
            display: flex;
            align-items: center;
            align-self: flex-start;
            padding: 0 32px;

            &__text {
                font-size: 14px;
                line-height: 20px;
                color: #56606d;
                margin-right: 9px;
                font-family: 'font_medium', sans-serif;
            }

            &__info {
                cursor: pointer;
                max-height: 16px;

                &__msg {
                    font-size: 12px;
                    line-height: 18px;
                    text-align: center;
                    color: #fff;

                    &__link {
                        font-size: 12px;
                        line-height: 18px;
                        color: #fff;
                        text-decoration: underline !important;

                        &:visited {
                            color: #fff;
                        }
                    }
                }
            }
        }

        &__address {
            box-sizing: border-box;
            display: flex;
            align-items: center;
            justify-content: space-between;
            border: 1px solid #c8d3de;
            border-radius: 8px;
            padding: 10px 15px;
            width: calc(100% - 64px);
            max-width: 482px;
            margin: 8px 0 15px;

            &__value {
                font-size: 14px;
                line-height: 20px;
                color: #000;
                white-space: nowrap;
                text-overflow: ellipsis;
                overflow: hidden;
            }

            &__copy-button {
                margin-left: 10px;
            }
        }

        &__footer {
            margin-top: 26px;
            background-color: #fec;
            padding: 12px 28px;
            border-radius: 0 0 10px 10px;
            width: calc(100% - 56px);

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 14px;
                line-height: 20px;
            }

            &__msg {
                font-size: 12px;
                line-height: 16px;
            }
        }
    }

    :deep(.info__box) {
        width: 214px;
        left: calc(50% - 107px);
        top: calc(100% - 80px);
        cursor: default;
        filter: none;
        transform: rotate(-180deg);
    }

    :deep(.info__box__message) {
        background: #56606d;
        border-radius: 4px;
        padding: 10px 8px;
        transform: rotate(-180deg);
    }

    :deep(.info__box__arrow) {
        background: #56606d;
        width: 10px;
        height: 10px;
        margin-bottom: -3px;
    }
</style>
