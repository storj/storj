// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container" />
</template>

<script lang="ts">
import { Vue } from 'vue-property-decorator';
export default class VerticalArrows extends Vue {
}
</script>

<style scoped lang="scss">
    .container {
        display: flex;
        flex-direction: column;
        margin-left: 10px;
        justify-content: space-between;
        height: 17px;
    }

    .inactive {
        opacity: 0.5;
    }

    .active {
        opacity: 1;

        &__fill {

            ::v-deep path {
                fill: #2683ff !important;
            }
        }
    }

</style>
