// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./overviewStep.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import FirstStepIcon from '@/../static/images/common/one.svg';
import ThirdStepIcon from '@/../static/images/common/three.svg';
import SecondStepIcon from '@/../static/images/common/two.svg';

@Component({
    components: {
        VButton,
        FirstStepIcon,
        ThirdStepIcon,
        SecondStepIcon,
    },
})
export default class OverviewStep extends Vue {
    /**
     * Holds button click logic.
     * Sets tour state to adding payment method state.
     */
    public onClick(): void {
        this.$emit('setAddPaymentState');
    }
}
</script>

<style scoped lang="scss" src="./overviewStep.scss"></style>
