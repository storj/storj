// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="success-step">
        <SuccessIcon class="success-step__big-icon" />
        <SmallSuccessIcon class="success-step__small-icon" />
        <h1 class="success-step__title">Success</h1>
        <p class="success-step__info">
            Your encryption passphrase is ready to use. Now you can upload files into your buckets securely using an
            encryption only you know.
        </p>
    </div>
</template>

<script setup lang="ts">
import SuccessIcon from '@/../static/images/projectPassphrase/success.svg';
import SmallSuccessIcon from '@/../static/images/projectPassphrase/smallSuccess.svg';
</script>

<style scoped lang="scss">
.success-step {
    display: flex;
    flex-direction: column;
    align-items: center;
    font-family: 'font_regular', sans-serif;
    max-width: 433px;

    &__small-icon {
        display: none;

        @media screen and (max-width: 530px) {
            display: block;
        }
    }

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
        max-width: 376px;
    }
}

@media screen and (max-width: 530px) {

    .success-step__big-icon {
        display: none;
    }
}
</style>
