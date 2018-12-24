// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="item-container">
        <Checkbox />
        <p class="item-container__item">test</p>
        <p class="item-container__item">test</p>
        <p class="item-container__item">test</p>
        <p class="item-container__item">test</p>
        <p class="item-container__item">test</p>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Checkbox from '@/components/common/Checkbox.vue';

@Component({
    components: {
        Checkbox,
    },
    props: {},
})

export default class ApiKeysItem extends Vue {
}
</script>

<style scoped lang="scss">
    .item-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: space-between;
        height: 90px;
        cursor: pointer;
        transition: box-shadow .2s ease-out;
        padding-left: 20px;
        transition: all .2s ease;
        border-radius: 6px;

        &:last-child {
            margin-bottom: 120px;
        }

        &:hover {
            background: #fff;
            box-shadow: 0px 4px 4px rgba(231, 232, 238, 0.6);
        }

        &__item {
            width: 20%;
            font-family: 'montserrat_medium';
            font-size: 16px;
            margin: 0;

        }
    }
</style>