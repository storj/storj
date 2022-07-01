// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        Overview
        <router-view />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

// @vue/component
@Component({
    components: {
    },
})
export default class BillingArea extends Vue {

}
</script>