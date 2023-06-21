// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="generated-step">
        <div class="generated-step__header">
            <PassphraseGeneratedIcon />
            <h1 class="generated-step__header__title">Passphrase Generated</h1>
        </div>
        <PassphraseGeneratedStep
            :on-back="onBack"
            :on-continue="onContinue"
            :passphrase="passphrase"
            :name="name"
            is-project-passphrase
        />
    </div>
</template>

<script setup lang="ts">
import PassphraseGeneratedStep from '@/components/accessGrants/createFlow/steps/PassphraseGeneratedStep.vue';

import PassphraseGeneratedIcon from '@/../static/images/accessGrants/newCreateFlow/passphraseGenerated.svg';

const props = withDefaults(defineProps<{
    name: string,
    onBack: () => void,
    onContinue: () => void,
    passphrase?: string,
}>(), {
    passphrase: '',
});
</script>

<style scoped lang="scss">
.generated-step {
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
