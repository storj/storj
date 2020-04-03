// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./OverviewArea.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import FirstStepIcon from '@/../static/images/common/one.svg';
import ThirdStepIcon from '@/../static/images/common/three.svg';
import SecondStepIcon from '@/../static/images/common/two.svg';

import { RouteConfig } from '@/router';

@Component({
    components: {
        VButton,
        FirstStepIcon,
        ThirdStepIcon,
        SecondStepIcon,
    },
})
export default class OverviewArea extends Vue {
    public readonly billingPath: string = RouteConfig.Account.with(RouteConfig.Billing).path;
}
</script>

<style scoped lang="scss" src="./OverviewArea.scss"></style>
