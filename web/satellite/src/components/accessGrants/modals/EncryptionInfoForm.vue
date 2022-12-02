// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info">
        <EncryptInfoIcon />
        <h1 class="info__title">Encryption Information</h1>
        <p class="info__info">
            By generating S3 credentials, you are opting in to <a class="info__info__link" href="https://docs.storj.io/dcs/concepts/encryption-key/design-decision-server-side-encryption/" target="_blank" rel="noopener noreferrer">server-side encryption</a>.
        </p>
        <VCheckbox
            class="info__checkbox"
            label="Don’t show this again."
            @setData="toggleCheckbox"
        />
        <div class="info__buttons">
            <VButton
                label="Go Back"
                height="48px"
                border-radius="8px"
                :is-transparent="true"
                :on-press="onBackClick"
            />
            <VButton
                label="Continue"
                height="48px"
                border-radius="8px"
                :on-press="onContinueClick"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { LocalData } from '@/utils/localData';

import VButton from '@/components/common/VButton.vue';
import VCheckbox from '@/components/common/VCheckbox.vue';

import EncryptInfoIcon from '@/../static/images/accessGrants/encyptInfoIcon.svg';

// @vue/component
@Component({
    components: {
        EncryptInfoIcon,
        VButton,
        VCheckbox,
    },
})

export default class EncryptionInfoFormModal extends Vue {
    public isDontShow = false;

    /**
     * Toggles checkbox.
     */
    public toggleCheckbox(value: boolean): void {
        this.isDontShow = value;
    }

    /**
     * Holds on back button click logic.
     * Emits back event.
     */
    public onBackClick(): void {
        this.$emit('back');
    }

    /**
     * Holds on continue button click logic.
     * Emits continue event.
     */
    public onContinueClick(): void {
        if (this.isDontShow) {
            LocalData.setServerSideEncryptionModalHidden(true);
        }
        
        this.$emit('continue');
    }
}
</script>

<style scoped lang="scss">
    .info {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        padding: 32px;
        max-width: 350px;
        font-family: 'font_regular', sans-serif;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 36px;
            letter-spacing: -0.02em;
            color: #000;
            margin: 16px 0;
            text-align: left;
        }

        &__info {
            font-size: 16px;
            line-height: 24px;
            color: #1b2533;
            margin-bottom: 16px;
            text-align: left;

            &__link {
                color: #1b2533;
                text-decoration: underline !important;
                text-underline-position: under;

                &:visited {
                    color: #1b2533;
                }
            }
        }

        &__buttons {
            width: 100%;
            display: flex;
            align-items: center;
            column-gap: 8px;
            margin-top: 16px;

            @media screen and (max-width: 390px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 8px;
            }
        }
    }
</style>
