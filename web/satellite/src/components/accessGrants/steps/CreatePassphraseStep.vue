// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-passphrase">
        <BackIcon class="create-passphrase__back-icon" @click="onBackClick"/>
        <h1 class="create-passphrase__title">Encryption Passphrase</h1>
        <div class="create-passphrase__warning">
            <WarningIcon/>
            <p class="create-passphrase__warning__label" v-if="isGenerateState">Save Your Encryption Passphrase</p>
            <p class="create-passphrase__warning__label" v-else>Remember This Passphrase</p>
        </div>
        <div class="create-passphrase__choosing">
            <p class="create-passphrase__choosing__label">Choose Passphrase Type</p>
            <div class="create-passphrase__choosing__right">
                <p
                    class="create-passphrase__choosing__right__option left-option"
                    :class="{ active: isGenerateState }"
                    @click="onChooseGenerate"
                >
                    Generate Phrase
                </p>
                <p
                    class="create-passphrase__choosing__right__option"
                    :class="{ active: isCreateState }"
                    @click="onChooseCreate"
                >
                    Create Phrase
                </p>
            </div>
        </div>
        <div class="create-passphrase__value-area">
            <div class="create-passphrase__value-area__mnemonic" v-if="isGenerateState">
                <p class="create-passphrase__value-area__mnemonic__label">12-Word Mnemonic Passphrase:</p>
                <div class="create-passphrase__value-area__mnemonic__container">
                    <p class="create-passphrase__value-area__mnemonic__container__value">{{ passphrase }}</p>
                    <VButton
                        class="create-passphrase__value-area__mnemonic__container__button"
                        label="Copy"
                        width="66px"
                        height="30px"
                        :on-press="onCopyClick"
                    />
                </div>
            </div>
            <div class="create-passphrase__value-area__password" v-else>
                <HeaderedInput
                    class="create-passphrase__value-area__password__input"
                    label="Create Your Passphrase"
                    placeholder="Enter your passphrase here"
                    @setData="onChangePassphrase"
                    :error="errorMessage"
                />
            </div>
        </div>
        <VButton
            class="create-passphrase__next-button"
            label="Next"
            width="100%"
            height="48px"
            :on-press="onNextClick"
            :is-disabled="isLoading"
        />
    </div>
</template>

<script lang="ts">
import * as bip39 from 'bip39';
import { Component, Vue } from 'vue-property-decorator';

import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';
import WarningIcon from '@/../static/images/accessGrants/warning.svg';

import { RouteConfig } from '@/router';
import { MetaUtils } from '@/utils/meta';

@Component({
    components: {
        WarningIcon,
        BackIcon,
        VButton,
        HeaderedInput,
    },
})
export default class CreatePassphraseStep extends Vue {
    private key: string = '';
    private access: string = '';
    private worker: Worker;
    private isLoading: boolean = true;

    public isGenerateState: boolean = true;
    public isCreateState: boolean = false;
    public passphrase: string = '';
    public errorMessage: string = '';

    /**
     * Lifecycle hook after initial render.
     * Sets local key from props value.
     */
    public async mounted(): Promise<void> {
        if (!this.$route.params.key) {
            await this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);
        }

        this.key = this.$route.params.key;
        this.passphrase = bip39.generateMnemonic();
        this.worker = await new Worker('/static/static/wasm/webWorker.js');
        this.worker.onmessage = (event: MessageEvent) => {
            const data = event.data;
            if (data.error) {
                this.$notify.error(data.error);

                return;
            }

            this.access = data.value;

            this.$notify.success('Access Grant was generated successfully');
        };
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message);
        };

        this.isLoading = false;
    }

    /**
     * Changes state to generate passphrase.
     */
    public onChooseGenerate(): void {
        if (this.passphrase && this.isGenerateState) return;

        this.passphrase = bip39.generateMnemonic();
        this.isCreateState = false;
        this.isGenerateState = true;
    }

    /**
     * Changes state to create passphrase.
     */
    public onChooseCreate(): void {
        if (this.passphrase && this.isCreateState) return;

        this.errorMessage = '';
        this.passphrase = '';
        this.isCreateState = true;
        this.isGenerateState = false;
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
     * Changes passphrase data from input value.
     * @param value
     */
    public onChangePassphrase(value: string): void {
        this.passphrase = value.trim();
        this.errorMessage = '';
    }

    /**
     * Holds on next button click logic.
     * Generates access grant and redirects to next step.
     */
    public onNextClick(): void {
        if (!this.passphrase) {
            this.errorMessage = 'Passphrase can`t be empty';

            return;
        }

        this.isLoading = true;

        const satelliteName = MetaUtils.getMetaContent('satellite-name');

        this.worker.postMessage({
            'type': 'GenerateAccess',
            'apiKey': this.key,
            'passphrase': this.passphrase,
            'projectID': this.$store.getters.selectedProject.id,
            'satelliteName': satelliteName,
        });

        // Give time for web worker to return value.
        setTimeout(() => {
            this.isLoading = false;

            this.$router.push({
                name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.ResultStep)).name,
                params: {
                    access: this.access,
                    key: this.key,
                },
            });
        }, 1000);
    }

    /**
     * Holds on back button click logic.
     * Redirects to previous step.
     */
    public onBackClick(): void {
        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.PermissionsStep)).name,
            params: {
                key: this.key,
            },
        });
    }
}
</script>

<style scoped lang="scss">
    .create-passphrase {
        height: calc(100% - 60px);
        padding: 30px 65px;
        max-width: 475px;
        min-width: 475px;
        font-family: 'font_regular', sans-serif;
        font-style: normal;
        display: flex;
        flex-direction: column;
        align-items: center;
        position: relative;
        background-color: #fff;
        border-radius: 0 6px 6px 0;

        &__back-icon {
            position: absolute;
            top: 30px;
            left: 65px;
            cursor: pointer;
        }

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
            align-items: center;
            padding: 20px;
            width: calc(100% - 40px);
            background: #fff9f7;
            border: 1px solid #f84b00;
            margin-bottom: 35px;
            border-radius: 8px;

            &__label {
                font-style: normal;
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 19px;
                color: #1b2533;
                margin: 0 0 0 15px;
            }
        }

        &__choosing {
            display: flex;
            align-items: center;
            justify-content: space-between;
            width: 100%;

            &__label {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 21px;
                color: #354049;
                margin: 0;
            }

            &__right {
                display: flex;
                align-items: center;

                &__option {
                    font-size: 14px;
                    line-height: 17px;
                    color: #768394;
                    margin: 0;
                    cursor: pointer;
                    border-bottom: 3px solid #fff;
                }
            }
        }

        &__value-area {
            margin: 50px 0 60px 0;
            min-height: 100px;
            width: 100%;
            display: flex;
            align-items: flex-start;

            &__mnemonic {

                &__label {
                    font-family: 'font_bold', sans-serif;
                    font-size: 14px;
                    line-height: 19px;
                    color: #7c8794;
                    margin: 0 0 10px 0;
                }

                &__container {
                    display: flex;
                    align-items: flex-start;
                    background: #f5f6fa;
                    border-radius: 9px;
                    padding: 10px;
                    width: calc(100% - 20px);

                    &__value {
                        font-family: 'Source Code Pro', sans-serif;
                        font-size: 14px;
                        line-height: 25px;
                        color: #384b65;
                        word-break: break-word;
                        margin: 0;
                        word-spacing: 8px;
                    }

                    &__button {
                        margin-left: 10px;
                        min-width: 66px;
                        min-height: 30px;
                    }
                }
            }

            &__password {
                width: 100%;

                &__input {
                    width: calc(100% - 8px);
                }
            }
        }
    }

    .left-option {
        margin-right: 15px;
    }

    .active {
        font-family: 'font_bold', sans-serif;
        color: #0068dc;
        border-bottom: 3px solid #0068dc;
    }

    /deep/ .label-container {

        &__main {
            margin-bottom: 10px;

            &__label {
                margin: 0;
                font-size: 14px;
                line-height: 19px;
                color: #7c8794;
                font-family: 'font_bold', sans-serif;
            }

            &__error {
                margin: 0 0 0 10px;
                font-size: 14px;
                line-height: 19px;
            }
        }
    }
</style>
