// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="verified-step">
        <VerifiedImage/>
        <h2 class="verified-step__title">Payment Verified</h2>
        <span class="verified-step__sub-title">
            Your payment has been successfully verified. Click the button below to name your project and start uploading data.
        </span>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VerifiedImage from '@/../static/images/onboardingTour/verified.svg';

@Component({
    components: {
        VerifiedImage,
    },
})

export default class VerifiedStep extends Vue {}
</script>

<style scoped lang="scss">
    .verified-step {
        width: 60%;
        padding-top: 30px;
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;

        &__title {
            font-size: 18px;
            line-height: 22px;
            color: #354049;
        }

        &__sub-title {
            font-size: 12px;
            line-height: 17px;
            text-align: center;
            color: #354049;
        }
    }
</style>
