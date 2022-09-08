// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="value-copy">
        <p class="value-copy__value" :aria-roledescription="roleDescription">{{ value }}</p>
        <VButton
            class="value-copy__button"
            label="Copy"
            width="66px"
            height="30px"
            is-blue-white="true"
            :on-press="onCopyClick"
        />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

// @vue/component
@Component({
    components: {
        VButton,
    },
})
export default class ValueWithCopy extends Vue {
    @Prop({ default: '' })
    public readonly value: string;
    @Prop({ default: '' })
    public readonly label: string;
    @Prop({ default: '' })
    public readonly roleDescription: string;

    /**
     * Holds on copy button click logic.
     * Copies value to clipboard.
     */
    public onCopyClick(): void {
        this.$copyText(this.value);
        this.$notify.success(`${this.label} was copied successfully`);
    }
}
</script>

<style scoped lang="scss">
    .value-copy {
        display: flex;
        align-items: center;
        padding: 12px 25px;
        background: #eff0f7;
        border-radius: 10px;
        max-width: calc(100% - 50px);

        &__value {
            font-size: 16px;
            line-height: 28px;
            color: #384b65;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        &__button {
            margin-left: 32px;
            min-width: 66px;
        }
    }
</style>
