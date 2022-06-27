// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <TopArrowIcon :class="`${isActive?'active':'inactive'}${direction.includes('ascending') && isActive?'__fill':''}`" />
        <BottomArrowIcon :class="`${isActive?'active':'inactive'}${direction.includes('descending') && isActive?'__fill':''}`" />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import BottomArrowIcon from '@/../static/images/common/bottomArrow.svg';
import TopArrowIcon from '@/../static/images/common/topArrow.svg';


// @vue/component
@Component({
    components: {
        TopArrowIcon,
        BottomArrowIcon,
    },
})
export default class VerticalArrows extends Vue {
    @Prop({default: false})
    private isActive: boolean;
    @Prop({default: ''})
    private direction: string;



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
