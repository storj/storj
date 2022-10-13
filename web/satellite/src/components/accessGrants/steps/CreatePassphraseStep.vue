// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-passphrase">
        <BackIcon class="create-passphrase__back-icon" @click="onBackClick" />
        <div class="create-passphrase__container">
            <h1 class="create-passphrase__container__title">Encryption Passphrase</h1>
            <div class="create-passphrase__container__choosing">
                <p class="create-passphrase__container__choosing__label">Passphrase</p>
                <div class="create-passphrase__container__choosing__right">
                    <p
                        class="create-passphrase__container__choosing__right__option left-option"
                        :class="{ active: isGenerateState }"
                        @click="onChooseGenerate"
                    >
                        Generate Phrase
                    </p>
                    <p
                        class="create-passphrase__container__choosing__right__option"
                        :class="{ active: isEnterState }"
                        @click="onChooseCreate"
                    >
                        Enter Phrase
                    </p>
                </div>
            </div>
            <div v-if="isEnterState" class="create-passphrase__container__enter-passphrase-box">
                <div class="create-passphrase__container__enter-passphrase-box__header">
                    <GreenWarningIcon />
                    <h2 class="create-passphrase__container__enter-passphrase-box__header__label">Enter an Existing Passphrase</h2>
                </div>
                <p class="create-passphrase__container__enter-passphrase-box__message">
                    if you already have an encryption passphrase, enter your encryption passphrase here.
                </p>
            </div>
            <div class="create-passphrase__container__value-area">
                <div v-if="isGenerateState" class="create-passphrase__container__value-area__mnemonic">
                    <p class="create-passphrase__container__value-area__mnemonic__value">{{ passphrase }}</p>
                    <VButton
                        class="create-passphrase__container__value-area__mnemonic__button"
                        label="Copy"
                        width="66px"
                        height="30px"
                        :on-press="onCopyClick"
                    />
                </div>
                <div v-else class="create-passphrase__container__value-area__password">
                    <VInput
                        placeholder="Enter encryption passphrase here"
                        :error="errorMessage"
                        @setData="onChangePassphrase"
                    />
                </div>
            </div>
            <div v-if="isGenerateState" class="create-passphrase__container__warning">
                <h2 class="create-passphrase__container__warning__title">Save Your Encryption Passphrase</h2>
                <p class="create-passphrase__container__warning__message">
                    You’ll need this passphrase to access data in the future. This is the only time it will be displayed.
                    Be sure to write it down.
                </p>
                <label class="create-passphrase__container__warning__check-area" :class="{ error: isError }" for="pass-checkbox">
                    <input
                        id="pass-checkbox"
                        v-model="isChecked"
                        class="create-passphrase__container__warning__check-area__checkbox"
                        type="checkbox"
                        @change="isError = false"
                    >
                    Yes, I wrote this down or saved it somewhere.
                </label>
            </div>
            <VButton
                class="create-passphrase__container__next-button"
                label="Next"
                width="100%"
                height="48px"
                :on-press="onNextClick"
                :is-disabled="isButtonDisabled"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { generateMnemonic } from 'bip39';

import { RouteConfig } from '@/router';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';

import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';
import GreenWarningIcon from '@/../static/images/accessGrants/greenWarning.svg';

// @vue/component
@Component({
    components: {
        BackIcon,
        GreenWarningIcon,
        VButton,
        VInput,
    },
})
export default class CreatePassphraseStep extends Vue {
    private key = '';
    private restrictedKey = '';
    private access = '';
    private worker: Worker;
    private isLoading = true;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public isGenerateState = true;
    public isEnterState = false;
    public isChecked = false;
    public isError = false;
    public passphrase = '';
    public errorMessage = '';

    /**
     * Lifecycle hook after initial render.
     * Sets local key from props value.
     */
    public async mounted(): Promise<void> {
        if (!this.$route.params.key && !this.$route.params.restrictedKey) {
            this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);
            await this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);

            return;
        }

        this.key = this.$route.params.key;
        this.restrictedKey = this.$route.params.restrictedKey;

        this.setWorker();

        this.passphrase = generateMnemonic();

        this.isLoading = false;
    }

    /**
     * Sets local worker with worker instantiated in store.
     * Also sets worker's onmessage and onerror logic.
     */
    public setWorker(): void {
        this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message);
        };
    }

    /**
     * Sets passphrase from child component.
     */
    public setPassphrase(passphrase: string): void {
        this.passphrase = passphrase;
    }

    /**
     * Holds on next button click logic.
     * Generates access grant and redirects to next step.
     */
    public async onNextClick(): Promise<void> {
        if (!this.passphrase) {
            this.errorMessage = 'Passphrase can\'t be empty';

            return;
        }

        if (!this.isChecked && this.isGenerateState) {
            this.isError = true;

            return;
        }

        if (this.isLoading) return;

        this.isLoading = true;

        await this.analytics.eventTriggered(AnalyticsEvent.PASSPHRASE_CREATED);

        const satelliteNodeURL: string = MetaUtils.getMetaContent('satellite-nodeurl');

        this.worker.postMessage({
            'type': 'GenerateAccess',
            'apiKey': this.restrictedKey,
            'passphrase': this.passphrase,
            'projectID': this.$store.getters.selectedProject.id,
            'satelliteNodeURL': satelliteNodeURL,
        });

        const accessEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        if (accessEvent.data.error) {
            await this.$notify.error(accessEvent.data.error);
            this.isLoading = false;

            return;
        }

        this.access = accessEvent.data.value;
        await this.$notify.success('Access Grant was generated successfully');

        this.isLoading = false;

        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.ResultStep)).path);
        await this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.ResultStep)).name,
            params: {
                access: this.access,
                key: this.key,
                restrictedKey: this.restrictedKey,
            },
        });
    }

    /**
     * Changes state to generate passphrase.
     */
    public onChooseGenerate(): void {
        if (this.passphrase && this.isGenerateState) return;

        this.passphrase = generateMnemonic();

        this.isEnterState = false;
        this.isGenerateState = true;
    }

    /**
     * Changes state to create passphrase.
     */
    public onChooseCreate(): void {
        if (this.passphrase && this.isEnterState) return;

        this.errorMessage = '';
        this.passphrase = '';

        this.isEnterState = true;
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
     * Indicates if button is disabled.
     */
    public get isButtonDisabled(): boolean {
        return this.isLoading || !this.passphrase || (!this.isChecked && this.isGenerateState);
    }

    /**
     * Holds on back button click logic.
     * Redirects to previous step.
     */
    public onBackClick(): void {
        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.PermissionsStep)).path);
        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.PermissionsStep)).name,
            params: {
                key: this.key,
            },
        });
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    private get isOnboardingTour(): boolean {
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
    }
}
</script>

<style scoped lang="scss">
    .create-passphrase {
        position: relative;

        &__back-icon {
            position: absolute;
            top: 30px;
            left: 65px;
            cursor: pointer;
        }

        &__container {
            padding: 25px 50px;
            max-width: 515px;
            min-width: 515px;
            font-family: 'font_regular', sans-serif;
            font-style: normal;
            display: flex;
            flex-direction: column;
            align-items: center;
            background-color: #fff;
            border-radius: 6px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-weight: bold;
                font-size: 22px;
                line-height: 27px;
                color: #000;
                margin: 0 0 30px;
            }

            &__enter-passphrase-box {
                padding: 20px;
                background: #f9fffc;
                border: 1px solid #1a9666;
                border-radius: 9px;

                &__header {
                    display: flex;
                    align-items: center;
                    margin-bottom: 10px;

                    &__label {
                        font-family: 'font_bold', sans-serif;
                        font-size: 16px;
                        line-height: 19px;
                        color: #1b2533;
                        margin: 0 0 0 10px;
                    }
                }

                &__message {
                    font-size: 16px;
                    line-height: 19px;
                    color: #1b2533;
                    margin: 0;
                }
            }

            &__warning {
                display: flex;
                flex-direction: column;
                padding: 20px;
                width: calc(100% - 40px);
                margin: 35px 0;
                background: #fff;
                border: 1px solid #e6e9ef;
                border-radius: 9px;

                &__title {
                    width: 100%;
                    text-align: center;
                    font-family: 'font_bold', sans-serif;
                    font-size: 16px;
                    line-height: 19px;
                    color: #1b2533;
                    margin: 0 0 0 15px;
                }

                &__message {
                    font-size: 16px;
                    line-height: 19px;
                    color: #1b2533;
                    margin: 10px 0 0;
                    text-align: center;
                }

                &__check-area {
                    margin-top: 27px;
                    font-size: 14px;
                    line-height: 19px;
                    color: #1b2533;
                    display: flex;
                    justify-content: center;
                    align-items: center;

                    &__checkbox {
                        margin: 0 10px 0 0;
                    }
                }
            }

            &__choosing {
                display: flex;
                align-items: center;
                justify-content: space-between;
                width: 100%;
                margin-bottom: 25px;

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
                width: 100%;
                display: flex;
                align-items: flex-start;

                &__mnemonic {
                    display: flex;
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

                &__password {
                    width: 100%;
                    margin: 10px 0 20px;
                }
            }
        }
    }

    .left-option {
        margin-right: 15px;
    }

    .active {
        font-family: 'font_medium', sans-serif;
        color: #0068dc;
        border-bottom: 3px solid #0068dc;
    }

    .error {
        color: red;
    }

    :deep(.label-container__main) {
        margin-bottom: 10px;
    }

    :deep(.label-container__main__label) {
        margin: 0;
        font-size: 14px;
        line-height: 19px;
        color: #7c8794;
        font-family: 'font_bold', sans-serif;
    }

    :deep(.label-container__main__error) {
        margin: 0 0 0 10px;
        font-size: 14px;
        line-height: 19px;
    }
</style>
