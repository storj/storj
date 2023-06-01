// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="passphrase-mode">
        <div class="passphrase-mode__header">
            <AccessEncryptionIcon />
            <h1 class="passphrase-mode__header__title">Encryption Passphrase</h1>
        </div>
        <p class="passphrase-mode__info">
            The encryption passphrase will be used to encrypt the files you upload in this project. You can generate
            a new encryption passphrase, or enter your own.
        </p>
        <ContainerWithIcon :icon-and-title="FUNCTIONAL_CONTAINER_ICON_AND_TITLE[FunctionalContainer.EncryptionPassphrase]">
            <template #functional>
                <div class="passphrase-mode__radios">
                    <Radio
                        id="generate passphrase"
                        :checked="isGenerate"
                        :on-check="setGenerate"
                        label="Generate 12-word passphrase"
                        info="Create this access with a new encryption passphrase that will be generated for you on
                            the next step. The access will not be able to manage any existing data."
                    />
                    <Radio
                        id="new passphrase"
                        :checked="!isGenerate"
                        :on-check="setEnter"
                        label="Enter a new passphrase"
                        info="Create this access with a new encryption passphrase that you can enter on the next step.
                            The access will not be able to manage any existing data."
                    />
                </div>
            </template>
        </ContainerWithIcon>
        <ButtonsContainer>
            <template #leftButton>
                <VButton
                    label="Cancel"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    border-radius="10px"
                    :on-press="onCancel"
                    :is-white="true"
                />
            </template>
            <template #rightButton>
                <VButton
                    label="Continue ->"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    border-radius="10px"
                    :on-press="onContinue"
                />
            </template>
        </ButtonsContainer>
    </div>
</template>

<script setup lang="ts">
import {
    FUNCTIONAL_CONTAINER_ICON_AND_TITLE,
    FunctionalContainer,
} from '@/types/createAccessGrant';

import ContainerWithIcon from '@/components/accessGrants/createFlow/components/ContainerWithIcon.vue';
import Radio from '@/components/accessGrants/createFlow/components/Radio.vue';
import ButtonsContainer from '@/components/accessGrants/createFlow/components/ButtonsContainer.vue';
import VButton from '@/components/common/VButton.vue';

import AccessEncryptionIcon from '@/../static/images/accessGrants/newCreateFlow/accessEncryption.svg';

const props = withDefaults(defineProps<{
    isGenerate?: boolean
    setGenerate?: () => void
    setEnter?: () => void
    onContinue?: () => void
    onCancel?: () => void
}>(), {
    isGenerate: true,
    setGenerate: () => () => {},
    setEnter: () => () => {},
    onContinue: () => () => {},
    onCancel: () => () => {},
});
</script>

<style scoped lang="scss">
.passphrase-mode {
    display: flex;
    flex-direction: column;
    font-family: 'font_regular', sans-serif;
    max-width: 350px;

    &__header {
        display: flex;
        align-items: center;
        padding-bottom: 16px;
        margin-bottom: 16px;
        border-bottom: 1px solid var(--c-grey-2);

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
            color: var(--c-grey-8);
            margin-left: 16px;
            text-align: left;
        }
    }

    &__info {
        font-size: 14px;
        line-height: 19px;
        color: var(--c-blue-6);
        padding-bottom: 16px;
        border-bottom: 1px solid var(--c-grey-2);
        text-align: left;
    }

    &__radios {
        display: flex;
        flex-direction: column;
        row-gap: 16px;
    }
}
</style>
