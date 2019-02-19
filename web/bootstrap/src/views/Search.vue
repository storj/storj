// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <h1>enter node ID</h1>
        <input type="text" v-model="nodeId"/>
        <button v-on:click="isNodeUp"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { isNodeUp } from '@/api/bootstrap';

@Component({
    data: function () {
        return {
            nodeId: '',
            status: ''
        };
    },
    methods: {
        isNodeUp: async function() {
            this.$data.status = await isNodeUp(this.$data.nodeId)
                ? 'online' : 'offline';
        }
    }
})

export default class Search extends Vue {}

</script>
