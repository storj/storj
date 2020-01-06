// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="content">
        <SNOContentTitle/>
        <SNOContentFilling/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import SNOContentFilling from '@/app/components/SNOContentFilling.vue';
import SNOContentTitle from '@/app/components/SNOContentTitle.vue';

import { NODE_ACTIONS } from '@/app/store/modules/node';

const {
    GET_NODE_INFO,
    SELECT_SATELLITE,
} = NODE_ACTIONS;

@Component ({
    components: {
        SNOContentTitle,
        SNOContentFilling,
    },
})
export default class Dashboard extends Vue {
    public async mounted() {
        await this.$store.dispatch(GET_NODE_INFO);
        await this.$store.dispatch(SELECT_SATELLITE, null);
    }
}
</script>

<style scoped lang="scss">
    .content {
        width: 822px;
        padding-top: 28px;
    }
</style>
