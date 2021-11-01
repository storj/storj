// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="encrypt">
        <div class="encrypt__msg-container">
            <p class="encrypt__msg-container__title" aria-roledescription="objects-title">
                The object browser uses
                <a
                    class="encrypt__msg-container__title__link"
                    href="https://docs.storj.io/dcs/concepts/encryption-key/design-decision-server-side-encryption"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    server side encryption.
                </a>
            </p>
            <p class="encrypt__msg-container__text">
                If you want to use our product with only end-to-end encryption, you may want to use our
                <span class="encrypt__msg-container__text__link" @click="navigateToCLIFlow">command line solution.</span>
            </p>
        </div>
        <GeneratePassphrase
            :on-next-click="onNextClick"
            :set-parent-passphrase="setPassphrase"
            :is-loading="isLoading"
        />
        <div class="encrypt__faq">
            <h2 class="encrypt__faq__title">FAQ</h2>
            <FAQBullet
                title="Why do I need a passphrase to upload?"
                text="One very important design consideration is that data stored on Storj is encrypted. That means
                only you have the encryption passphrase for your data. The service doesn't ever have access to or store
                your encryption passphrase. If you lose your passphrase, you will be unable to recover your data."
            />
            <FAQBullet
                title="What if I enter a wrong passphrase?"
                text="There is no wrong passphrase because every passphrase can have access to different files. Entering
                a new or different passphrase won’t have any effect on the existing files. If you enter your passphrase
                and don’t see the existing files, it’s most likely you entered a new passphrase and that’s why you can’t
                see the encrypted data stored with a different passphrase."
            />
            <FAQBullet
                title="Why I have to enter a passphrase for every bucket?"
                text="In general, the best practice is to use one encryption passphrase per bucket. If an object with
                the same path and object name uploaded by two uplinks with encryption keys derived from the same
                encryption passphrase, the most recent upload will over-write the older object."
            />
            <FAQBullet
                title="Why there is no “remember passphrase”?"
                text="There is no wrong passphrase because every passphrase can have access to different files. Entering
                a new or different passphrase won’t have any effect on the existing files. If you enter a passphrase and
                don’t see your existing files, try to enter your passphrase again."
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import pbkdf2 from 'pbkdf2';

import { RouteConfig } from '@/router';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { LocalData } from "@/utils/localData";

import GeneratePassphrase from "@/components/common/GeneratePassphrase.vue";
import FAQBullet from "@/components/objects/FAQBullet.vue";

// @vue/component
@Component({
    components: {
        GeneratePassphrase,
        FAQBullet,
    },
})
export default class EncryptData extends Vue {
    public isLoading = false;
    public passphrase = '';

    /**
     * Sets passphrase from child component.
     */
    public navigateToCLIFlow(): void {
        this.$router.push({
            name: RouteConfig.APIKey.name,
            params: {
                backRoute: this.$route.path,
            }
        })
    }

    /**
     * Sets passphrase from child component.
     */
    public setPassphrase(passphrase: string): void {
        this.passphrase = passphrase;
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        const SALT = 'storj-unique-salt';

        const result: Buffer | Error = await this.pbkdf2Async(SALT);

        if (result instanceof Error) {
            await this.$notify.error(result.message);

            return;
        }

        const keyToBeStored = await result.toString('hex');

        await LocalData.setUserIDPassSalt(this.$store.getters.user.id, keyToBeStored, SALT);
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
        padding-bottom: 30px;

        &__msg-container {
            margin: -20px auto 40px auto;
            max-width: 620px;
            background-color: #ffd78a;
            display: flex;
            flex-direction: column;
            align-items: center;
            border-radius: 0 0 10px 10px;
            padding: 20px 0;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 19px;
                text-align: center;
                color: #000;
                margin-bottom: 10px;

                &__link {
                    color: #000;
                    text-decoration: underline !important;
                    text-underline-position: under;

                    &:visited {
                        color: #000;
                    }
                }
            }

            &__text {
                font-size: 14px;
                line-height: 20px;
                text-align: center;
                color: #1b2533;
                max-width: 400px;

                &__link {
                    color: #1b2533;
                    text-decoration: underline;
                    text-underline-position: under;
                    cursor: pointer;
                }
            }
        }

        &__faq {
            max-width: 620px;
            // display: flex; revert this when FAQ content will be confirmed
            display: none;
            flex-direction: column;
            align-items: center;
            margin: 0 auto;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 36px;
                line-height: 56px;
                letter-spacing: 1px;
                color: #14142b;
                margin: 75px 0 30px 0;
            }
        }
    }
</style>
