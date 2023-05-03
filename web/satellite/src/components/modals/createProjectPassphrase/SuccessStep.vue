// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="success-step">
        <div class="success-step__header">
            <SuccessIcon />
            <h1 class="success-step__header__title">Success</h1>
        </div>
        <p class="success-step__info">
            Your encryption passphrase is ready to use. Now you can upload files into your buckets securely using an
            encryption passphrase only you know.
        </p>
        <VButton
            label="Continue ->"
            width="100%"
            height="40px"
            font-size="14px"
            border-radius="10px"
            :on-press="onContinue"
        />
    </div>
</template>

<script setup lang="ts">
import VButton from '@/components/common/VButton.vue';

import SuccessIcon from '@/../static/images/accessGrants/newCreateFlow/accessCreated.svg';

const props = defineProps<{
    onContinue: () => void
}>();
</script>

<style scoped lang="scss">
.success-step {
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
        }
    }

    &__info {
        font-size: 14px;
        line-height: 19px;
        color: var(--c-blue-6);
        padding: 16px 0;
        margin-bottom: 16px;
        border-bottom: 1px solid var(--c-grey-2);
        text-align: left;
    }
}
</style>
