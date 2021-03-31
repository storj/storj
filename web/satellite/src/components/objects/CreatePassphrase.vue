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
    public onNextClick(): void {
        if (this.isLoading) return;

        this.isLoading = true;

        const SALT = 'storj-unique-salt';
        pbkdf2.pbkdf2(this.passphrase, SALT, 1, 64, (error, key) => {
            if (error) return this.$notify.error(error.message);

            LocalData.setUserIDPassSalt(this.$store.getters.user.id, key.toString('hex'), SALT);
        });

        this.isLoading = false;

        this.$store.dispatch(OBJECTS_ACTIONS.SET_PASSPHRASE, this.passphrase);
        this.$router.push({name: RouteConfig.BucketsManagement.name});
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
