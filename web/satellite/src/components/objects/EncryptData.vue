// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="encrypt">
        <h1 class="encrypt__title">Objects</h1>
        <div class="encrypt__container">
            <EncryptIcon />
            <h1 class="encrypt__container__title">Encrypt your data</h1>
            <p class="encrypt__container__info">
                The encryption passphrase is used to encrypt and access the data that you upload to Storj DCS.
            </p>
            <div class="encrypt__container__header">
                <p class="encrypt__container__header__rec">RECOMMENDED</p>
                <div class="encrypt__container__header__row">
                    <p class="encrypt__container__header__row__gen" :class="{ active: isGenerate }" @click="setToGenerate">Generate Phrase</p>
                    <div class="encrypt__container__header__row__right">
                        <p class="encrypt__container__header__row__right__enter" :class="{ active: !isGenerate }" @click="setToEnter">Enter Your Own Passphrase</p>
                        <VInfo
                            class="encrypt__container__header__row__right__info-button"
                            text="We strongly encourage you to use a mnemonic phrase, which is automatically generated one on the client-side for you. Alternatively, you can enter your own passphrase."
                        >
                            <InfoIcon class="encrypt__container__header__row__right__info-button__image" />
                        </VInfo>
                    </div>
                </div>
            </div>
            <div v-if="isGenerate" class="encrypt__container__generate">
                <p class="encrypt__container__generate__value">{{ passphrase }}</p>
                <VButton
                    class="encrypt__container__generate__button"
                    label="Copy"
                    width="66px"
                    height="30px"
                    is-blue-white="true"
                    :on-press="onCopyClick"
                />
            </div>
            <div v-else class="encrypt__container__enter">
                <HeaderlessInput
                    placeholder="Enter a passphrase here..."
                    width="100%"
                    :error="enterError"
                    @setData="setPassphrase"
                />
            </div>
            <div class="encrypt__container__save">
                <h2 class="encrypt__container__save__title">Save your encryption passphrase</h2>
                <p class="encrypt__container__save__msg">
                    Please note that Storj does not know or store your encryption passphrase. If you lose it, you will
                    not be able to recover your files.
                </p>
            </div>
            <div class="encrypt__container__buttons">
                <VButton
                    label="Next >"
                    width="100%"
                    height="64px"
                    border-radius="62px"
                    :on-press="onNextClick"
                />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import pbkdf2 from 'pbkdf2';
import * as bip39 from 'bip39';

import { RouteConfig } from '@/router';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { LocalData, UserIDPassSalt } from '@/utils/localData';

import VInfo from '@/components/common/VInfo.vue';
import VButton from '@/components/common/VButton.vue';
import HeaderlessInput from '@/components/common/HeaderlessInput.vue';

import EncryptIcon from '@/../static/images/objects/encrypt.svg';
import InfoIcon from '@/../static/images/common/greyInfo.svg';

// @vue/component
@Component({
    components: {
        EncryptIcon,
        InfoIcon,
        VInfo,
        VButton,
        HeaderlessInput,
    },
})
export default class EncryptData extends Vue {
    private isLoading = false;
    private keyToBeStored = '';

    public isGenerate = true;
    public enterError = '';
    public passphrase = '';

    /**
     * Lifecycle hook after initial render.
     * Chooses correct state and generates mnemonic.
     */
    public mounted(): void {
        const idPassSalt: UserIDPassSalt | null = LocalData.getUserIDPassSalt();
        if (idPassSalt && idPassSalt.userId === this.$store.getters.user.id) {
            this.isGenerate = false;

            return;
        }

        this.passphrase = bip39.generateMnemonic();
    }

    /**
     * Holds on copy button click logic.
     * Copies passphrase to clipboard.
     */
    public onCopyClick(): void {
        this.$copyText(this.passphrase);
        this.$notify.success('Passphrase was copied successfully');
    }

    /**
     * Sets passphrase from child component.
     */
    public setPassphrase(passphrase: string): void {
        if (this.enterError) this.enterError = '';

        this.passphrase = passphrase;
    }

    /**
     * Sets view state to enter passphrase.
     */
    public setToEnter(): void {
        this.passphrase = '';
        this.isGenerate = false;
    }

    /**
     * Sets view state to generate passphrase.
     */
    public setToGenerate(): void {
        if (this.enterError) this.enterError = '';

        this.passphrase = bip39.generateMnemonic();
        this.isGenerate = true;
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        if (this.isLoading) return;

        if (!this.passphrase) {
            this.enterError = 'Can\'t be empty';

            return;
        }

        this.isLoading = true;

        const SALT = 'storj-unique-salt';

        const result: Buffer | Error = await this.pbkdf2Async(SALT);

        if (result instanceof Error) {
            await this.$notify.error(result.message);

            return;
        }

        this.keyToBeStored = await result.toString('hex');

        await LocalData.setUserIDPassSalt(this.$store.getters.user.id, this.keyToBeStored, SALT);
        await this.$store.dispatch(OBJECTS_ACTIONS.SET_PASSPHRASE, this.passphrase);

        this.isLoading = false;

        await this.$router.push({name: RouteConfig.BucketsManagement.name});
    }

    /**
     * Generates passphrase fingerprint asynchronously.
     */
    private pbkdf2Async(salt: string): Promise<Buffer | Error> {
        const ITERATIONS = 1;
        const KEY_LENGTH = 64;

        return new Promise((response, reject) => {
            pbkdf2.pbkdf2(this.passphrase, salt, ITERATIONS, KEY_LENGTH, (error, key) => {
                error ? reject(error) : response(key);
            });
        });
    }
}
</script>

<style scoped lang="scss">
    .encrypt {
        font-family: 'font_regular', sans-serif;
        padding-bottom: 60px;

        &__title {
            font-family: 'font_medium', sans-serif;
            font-weight: bold;
            font-size: 18px;
            line-height: 26px;
            color: #232b34;
        }

        &__container {
            padding: 60px;
            max-width: 500px;
            background: #fcfcfc;
            box-shadow: 0 0 32px rgba(0, 0, 0, 0.04);
            border-radius: 20px;
            margin: 30px auto 0 auto;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 36px;
                line-height: 56px;
                letter-spacing: 1px;
                color: #14142b;
                margin: 35px 0 10px 0;
            }

            &__info {
                font-size: 16px;
                line-height: 32px;
                letter-spacing: 0.75px;
                color: #1b2533;
                margin-bottom: 20px;
            }

            &__header {

                &__rec {
                    font-size: 12px;
                    line-height: 15px;
                    color: #1b2533;
                    opacity: 0.4;
                    margin-bottom: 15px;
                }

                &__row {
                    display: flex;
                    align-items: center;
                    justify-content: space-between;

                    &__gen {
                        font-family: 'font_bold', sans-serif;
                        font-size: 16px;
                        line-height: 19px;
                        color: #a9b5c1;
                        padding-bottom: 10px;
                        border-bottom: 5px solid #fff;
                        cursor: pointer;
                    }

                    &__right {
                        display: flex;
                        align-items: flex-start;

                        &__enter {
                            font-family: 'font_bold', sans-serif;
                            font-size: 16px;
                            line-height: 19px;
                            color: #a9b5c1;
                            cursor: pointer;
                            margin-right: 10px;
                            padding-bottom: 10px;
                            border-bottom: 5px solid #fff;
                        }

                        &__info-button {

                            &__image {
                                cursor: pointer;
                            }
                        }
                    }
                }
            }

            &__generate {
                margin-top: 25px;
                display: flex;
                align-items: center;
                padding: 25px;
                background: #eff0f7;
                border-radius: 10px;

                &__value {
                    font-size: 16px;
                    line-height: 28px;
                    color: #384b65;
                }

                &__button {
                    margin-left: 32px;
                    min-width: 66px;
                }
            }

            &__enter {
                margin-top: 25px;
            }

            &__save {
                border: 1px solid #e6e9ef;
                border-radius: 10px;
                padding: 25px;
                margin-top: 35px;

                &__title {
                    font-family: 'font_bold', sans-serif;
                    font-size: 16px;
                    line-height: 19px;
                    color: #1b2533;
                    margin-bottom: 12px;
                }

                &__msg {
                    font-size: 14px;
                    line-height: 20px;
                    color: #1b2533;
                }
            }

            &__buttons {
                display: flex;
                align-items: center;
                margin-top: 30px;

                &__back {
                    margin-right: 24px;
                }
            }
        }
    }

    .active {
        color: #0149ff;
        border-color: #0149ff;
    }

    ::v-deep .info__box__message {
        min-width: 440px;

        &__regular-text {
            line-height: 32px;
        }
    }
</style>
