// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="enter-step">
        <LockIcon />
        <h1 class="enter-step__title">Enter Passphrase</h1>
        <p class="enter-step__info">
            Please note that Storj does not know or store your encryption passphrase. If you lose it, you will not be
            able to recover your files. Save your passphrase and keep it safe.
        </p>
        <VInput
            label="Encryption Passphrase"
            :is-password="true"
            width="100%"
            height="56px"
            placeholder="Enter Encryption Passphrase"
            :error="enterError"
            @setData="setPassphrase"
        />
    </div>
</template>

<script setup lang="ts">
import VInput from '@/components/common/VInput.vue';

import LockIcon from '@/../static/images/projectPassphrase/lock.svg';

const props = withDefaults(defineProps<{
    setPassphrase?: (v: string) => void,
    enterError?: string,
}>(), {
    setPassphrase: () => () => {},
    enterError: '',
});
</script>

<style scoped lang="scss">
.enter-step {
    display: flex;
    flex-direction: column;
    align-items: center;
    font-family: 'font_regular', sans-serif;
    max-width: 433px;

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 32px;
        line-height: 39px;
        color: #1b2533;
        margin: 14px 0;
    }

    &__info {
        font-size: 14px;
        line-height: 19px;
        color: #354049;
        margin-bottom: 24px;
    }
}
</style>
