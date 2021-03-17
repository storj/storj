// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-pass">
        <GeneratePassphrase
            :is-loading="isLoading"
            :on-button-click="onNextClick"
            :set-parent-passphrase="setPassphrase"
        />
    </div>
</template>

<script lang="ts">
import pbkdf2 from 'pbkdf2';
import { Component, Vue } from 'vue-property-decorator';

import GeneratePassphrase from '@/components/common/GeneratePassphrase.vue';

import { RouteConfig } from '@/router';
import { LocalData } from '@/utils/localData';

@Component({
    components: {
        GeneratePassphrase,
    },
})
export default class CreatePassphrase extends Vue {
    private isLoading: boolean = false;

    public passphrase: string = '';

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
        pbkdf2.pbkdf2(this.passphrase, SALT, 1, 64, (error, key) => {
            if (error) return this.$notify.error(error.message);

            LocalData.setUserIDPassSalt(this.$store.getters.user.id, key.toString('hex'), SALT);
        });

        this.isLoading = false;

        await this.$router.push(RouteConfig.UploadFile.path);
    }
}
</script>

<style scoped lang="scss">
    .create-pass {
        margin-top: 150px;
    }
</style>
