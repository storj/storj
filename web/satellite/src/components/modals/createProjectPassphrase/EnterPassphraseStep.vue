// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="enter-step">
        <div class="enter-step__header">
            <AccessEncryptionIcon />
            <h1 class="enter-step__header__title">Enter Passphrase</h1>
        </div>
        <EnterPassphraseStep
            is-new-passphrase
            is-project-passphrase
            :on-back="onBack"
            :on-continue="onContinue"
            :passphrase="passphrase"
            :set-passphrase="setPassphrase"
            info="Please note that Storj does not know or store your encryption passphrase. If you lose it, you will
                not be able to recover your files."
        />
    </div>
</template>

<script setup lang="ts">
import EnterPassphraseStep from '@/components/accessGrants/createFlow/steps/EnterPassphraseStep.vue';

import AccessEncryptionIcon from '@/../static/images/accessGrants/newCreateFlow/accessEncryption.svg';

const props = withDefaults(defineProps<{
    passphrase: string,
    setPassphrase?: (v: string) => void,
    onBack: () => void,
    onContinue: () => void,
}>(), {
    passphrase: '',
    setPassphrase: () => () => {},
    onBack: () => () => {},
    onContinue: () => () => {},
});
</script>

<style scoped lang="scss">
.enter-step {
    display: flex;
    flex-direction: column;
    font-family: 'font_regular', sans-serif;
    max-width: 350px;

    &__header {
        display: flex;
        align-items: center;
        padding-bottom: 16px;
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
}
</style>
