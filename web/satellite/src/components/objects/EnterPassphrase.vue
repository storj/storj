// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="enter-pass">
        <h1 class="enter-pass__title">Objects</h1>
        <div class="enter-pass__container">
            <h1 class="enter-pass__container__title">Access Data in Browser</h1>
            <div class="enter-pass__container__warning">
                <div class="enter-pass__container__warning__header">
                    <WarningIcon/>
                    <p class="enter-pass__container__warning__header__label">Would you like to access files in your browser?</p>
                </div>
                <p class="enter-pass__container__warning__message">
                    Entering your encryption passphrase here will share encryption data with your browser.
                </p>
            </div>
            <label class="enter-pass__container__textarea" for="enter-pass-textarea">
                <p class="enter-pass__container__textarea__label">Encryption Passphrase</p>
                <textarea
                    class="enter-pass__container__textarea__input"
                    :class="{ error: isError }"
                    id="enter-pass-textarea"
                    placeholder="Enter encryption passphrase here"
                    rows="2"
                    @input="onChangePassphrase"
                />
            </label>
            <div class="enter-pass__container__error" v-if="isError">
                <h2 class="enter-pass__container__error__title">Encryption Passphrase Does not Match</h2>
                <p class="enter-pass__container__error__message">
                    A previous fingerprint of a passphrase-based-key-derivation-function created in this browser doesn't
                    match the passphrase you just entered. Objects uploaded with a different encryption passphrase will
                    NOT be accessible.
                </p>
                <label class="enter-pass__container__error__check-area" :class="{ error: isCheckboxError }" for="error-checkbox">
                    <input
                        class="enter-pass__container__error__check-area__checkbox"
                        id="error-checkbox"
                        type="checkbox"
                        v-model="isCheckboxChecked"
                        @change="isCheckboxError = false"
                    >
                    I acknowledge this passphrase has not been used in this browser before.
                </label>
            </div>
            <VButton
                class="enter-pass__container__next-button"
                label="Access Data"
                width="100%"
                height="48px"
                :on-press="onAccessDataClick"
                :is-disabled="!passphrase"
            />
        </div>
    </div>
</template>

<script lang="ts">
import pbkdf2 from 'pbkdf2';
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import WarningIcon from '@/../static/images/common/greyWarning.svg';

import { RouteConfig } from '@/router';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { LocalData, UserIDPassSalt } from '@/utils/localData';
import { MetaUtils } from '@/utils/meta';

@Component({
    components: {
        VButton,
        WarningIcon,
    },
})
export default class EnterPassphrase extends Vue {
    private hashFromInput: string = '';

    public passphrase: string = '';
    public isError: boolean = false;
    public isCheckboxChecked: boolean = false;
    public isCheckboxError: boolean = false;

    /**
     * Lifecycle hook after initial render.
     * Chooses correct route.
     */
    public mounted(): void {
        const idPassSalt: UserIDPassSalt | null = LocalData.getUserIDPassSalt();
        if (!idPassSalt) {
            this.$router.push({name: RouteConfig.CreatePassphrase.name});
        }
    }

    /**
     * Changes passphrase data from input value.
     * @param event
     */
    public onChangePassphrase(event): void {
        this.passphrase = event.target.value.trim();
        this.resetErrors();
    }

    /**
     * Returns docs link from config.
     */
    public get docsLink(): string {
        return MetaUtils.getMetaContent('documentation-url');
    }

    /**
     * Holds on access data button click logic.
     */
    public async onAccessDataClick(): Promise<void> {
        if (!this.passphrase) return;

        const hashFromStorage: UserIDPassSalt | null = LocalData.getUserIDPassSalt();
        if (!hashFromStorage) return;

        const result: Buffer | Error = await this.pbkdf2Async(hashFromStorage.salt);

        if (result instanceof Error) {
            await this.$notify.error(result.message);

            return;
        }

        this.hashFromInput = await result.toString('hex');

        const areHashesEqual = () => {
            return hashFromStorage.passwordHash === this.hashFromInput;
        };

        switch (true) {
            case areHashesEqual() ||
            !areHashesEqual() && this.isError && this.isCheckboxChecked:
                await this.$store.dispatch(OBJECTS_ACTIONS.SET_PASSPHRASE, this.passphrase);
                await this.$router.push({name: RouteConfig.BucketsManagement.name});

                return;
            case !areHashesEqual() && this.isError && !this.isCheckboxChecked:
                this.isCheckboxError = true;

                return;
            case !areHashesEqual():
                this.isError = true;

                return;
            default:
        }
    }

    /**
     * Reset all error states to default.
     */
    public resetErrors(): void {
        this.isCheckboxError = false;
        this.isCheckboxChecked = false;
        this.isError = false;
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
    .enter-pass {
        display: flex;
        flex-direction: column;
        align-items: center;

        &__title {
            font-family: 'font_medium', sans-serif;
            font-style: normal;
            font-weight: bold;
            font-size: 18px;
            line-height: 26px;
            color: #232b34;
            margin: 0;
            width: 100%;
            text-align: left;
        }

        &__container {
            padding: 45px 50px 60px 50px;
            max-width: 515px;
            min-width: 515px;
            font-family: 'font_regular', sans-serif;
            font-style: normal;
            display: flex;
            flex-direction: column;
            align-items: center;
            background-color: #fff;
            border-radius: 6px;
            margin: 100px 0 30px 0;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-weight: bold;
                font-size: 22px;
                line-height: 27px;
                color: #000;
                margin: 0 0 30px 0;
            }

            &__warning {
                display: flex;
                flex-direction: column;
                padding: 20px;
                width: calc(100% - 40px);
                background: #f5f6fa;
                border: 1px solid #a9b5c1;
                border-radius: 8px;

                &__header {
                    display: flex;
                    align-items: center;

                    &__label {
                        font-style: normal;
                        font-family: 'font_bold', sans-serif;
                        font-size: 16px;
                        line-height: 19px;
                        color: #1b2533;
                        margin: 0 0 0 15px;
                    }
                }

                &__message {
                    font-size: 16px;
                    line-height: 19px;
                    color: #1b2533;
                    margin: 10px 0 0 0;
                }
            }

            &__textarea {
                width: 100%;
                font-size: 16px;
                line-height: 21px;
                color: #354049;
                margin: 26px 0 10px 0;

                &__label {
                    margin: 0 0 8px 0;
                }

                &__input {
                    padding: 15px 20px;
                    resize: none;
                    width: calc(100% - 42px);
                    font-size: 14px;
                    line-height: 25px;
                    border-radius: 6px;
                }
            }

            &__error {
                display: flex;
                flex-direction: column;
                align-items: flex-start;
                color: #ce3030;

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    line-height: 21px;
                    margin: 0 0 5px 0;
                }

                &__message {
                    font-weight: normal;
                    margin: 0 0 20px 0;
                }

                &__check-area {
                    margin-bottom: 32px;
                    font-size: 14px;
                    line-height: 19px;
                    color: #1b2533;
                    display: flex;
                    align-items: center;
                    cursor: pointer;

                    &__checkbox {
                        margin: 0 10px 0 0;
                    }
                }
            }
        }
    }

    .error {
        border-color: #ce3030;
        color: #ce3030;
    }
</style>
