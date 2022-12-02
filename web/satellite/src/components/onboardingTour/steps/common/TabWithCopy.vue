// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="tab-copy">
        <p class="tab-copy__value" :aria-roledescription="ariaRoleDescription">{{ value }}</p>
        <CopyIcon class="tab-copy__icon" @click="onCopyClick" />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import CopyIcon from '@/../static/images/common/copy.svg';

// @vue/component
@Component({
    components: {
        CopyIcon,
    },
})
export default class TabWithCopy extends Vue {
    @Prop({ default: '' })
    public readonly value: string;
    @Prop({ default: '' })
    public readonly ariaRoleDescription: string;

    /**
     * Holds on copy button click logic.
     * Copies command to clipboard.
     */
    public onCopyClick(): void {
        this.$copyText(this.value);
        this.$notify.success('Command was copied successfully');
    }
}
</script>

<style scoped lang="scss">
    .tab-copy {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 24px 30px;
        background: #183055;
        border-radius: 0 6px 6px;

        &__value {
            font-size: 14px;
            color: #e6ecf1;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        &__icon {
            margin-left: 24px;
            min-width: 13px;
            cursor: pointer;
        }
    }
</style>
