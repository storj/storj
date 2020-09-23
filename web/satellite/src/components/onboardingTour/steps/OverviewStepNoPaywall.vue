// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./overviewStepNoPaywall.html"></template>

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
export default class OverviewStepNoPaywall extends Vue {
    /**
     * Holds button click logic.
     * Sets tour state to adding project state.
     */
    public onClick(): void {
        this.$emit('setCreateProjectState');
    }
}
</script>

<style scoped lang="scss" src="./overviewStepNoPaywall.scss"></style>
