// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="enter-passphrase">
        <BackIcon class="enter-passphrase__back-icon" @click="onBackClick"/>
        <h1 class="enter-passphrase__title">Enter Encryption Passphrase</h1>
        <p class="enter-passphrase__sub-title">Enter the passphrase you most recently generated for Access Grants</p>
        <HeaderedInput
            class="enter-passphrase__input"
            label="Encryption Passphrase"
            placeholder="Enter your passphrase here"
            @setData="onChangePassphrase"
            :error="errorMessage"
        />
        <VButton
            class="enter-passphrase__next-button"
            label="Next"
            width="100%"
            height="48px"
            :on-press="onNextClick"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';

import { RouteConfig } from '@/router';

@Component({
    components: {
        HeaderedInput,
        VButton,
        BackIcon,
    },
})
export default class EnterPassphraseStep extends Vue {
    private key: string = '';

    public passphrase: string = '';
    public errorMessage: string = '';

    /**
     * Lifecycle hook after initial render.
     * Sets local key from props value.
     */
    public mounted(): void {
        if (!this.$route.params.key) {
            this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);
        }

        this.key = this.$route.params.key;
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

        // mock for now.
        return;
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
    .enter-passphrase {
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

        &__sub-title {
            font-weight: normal;
            font-size: 16px;
            line-height: 21px;
            color: #000;
            text-align: center;
            margin: 0 0 75px 0;
            max-width: 340px;
        }

        &__input {
            width: calc(100% - 8px);
        }

        &__next-button {
            margin-top: 93px;
        }
    }
</style>

