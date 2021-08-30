// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="encrypt">
        <h1 class="encrypt__title">Objects</h1>
        <GeneratePassphrase
            :on-next-click="onNextClick"
            :set-parent-passphrase="setPassphrase"
            :is-loading="isLoading"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import pbkdf2 from 'pbkdf2';

import { RouteConfig } from '@/router';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { LocalData } from "@/utils/localData";

import GeneratePassphrase from "@/components/common/GeneratePassphrase.vue";

// @vue/component
@Component({
    components: {
        GeneratePassphrase,
    },
})
export default class EncryptData extends Vue {
    public isLoading = false;
    public passphrase = '';

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
        padding-bottom: 60px;

        &__title {
            font-family: 'font_medium', sans-serif;
            font-weight: bold;
            font-size: 18px;
            line-height: 26px;
            color: #232b34;
        }
    }
</style>
