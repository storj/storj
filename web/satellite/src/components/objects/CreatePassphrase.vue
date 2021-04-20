// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-pass">
        <h1 class="create-pass__title">Objects</h1>
        <div class="create-pass__container">
            <GeneratePassphrase
                :is-loading="isLoading"
                :on-button-click="onNextClick"
                :set-parent-passphrase="setPassphrase"
            />
        </div>
    </div>
</template>

<script lang="ts">
import pbkdf2 from 'pbkdf2';
import { Component, Vue } from 'vue-property-decorator';

import GeneratePassphrase from '@/components/common/GeneratePassphrase.vue';

import { RouteConfig } from '@/router';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { LocalData, UserIDPassSalt } from '@/utils/localData';

@Component({
    components: {
        GeneratePassphrase,
    },
})
export default class CreatePassphrase extends Vue {
    private isLoading: boolean = false;
    private keyToBeStored: string = '';

    public passphrase: string = '';

    /**
     * Lifecycle hook after initial render.
     * Chooses correct route.
     */
    public mounted(): void {
        const idPassSalt: UserIDPassSalt | null = LocalData.getUserIDPassSalt();
        if (idPassSalt && idPassSalt.userId === this.$store.getters.user.id) {
            this.$router.push({name: RouteConfig.EnterPassphrase.name});
        }
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

        this.keyToBeStored = await result.toString('hex');

        await LocalData.setUserIDPassSalt(this.$store.getters.user.id, this.keyToBeStored, SALT);
        await this.$store.dispatch(OBJECTS_ACTIONS.SET_PASSPHRASE, this.passphrase);

        this.isLoading = false;

        await this.$router.push({name: RouteConfig.EnterPassphrase.name});
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
    .create-pass {
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
            margin-top: 100px;
        }
    }
</style>
