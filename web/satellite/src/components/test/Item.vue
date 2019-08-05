// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
            <td>{{data.a }}</td>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';

    @Component({
        components: {
        },
    })
    export default class Item extends Vue {
        @Prop()
        private readonly data: string;

    }
</script>

<style scoped lang="scss">

</style>
