// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr
        :class="{ 'selected': selected }"
        @click="onClick"
    >
        <th v-if="selectable" class="icon select">
            <v-table-checkbox :disabled="selectDisabled" :value="selected" @checkChange="onChange" />
        </th>
        <th v-for="(val, key, index) in item" :key="index" class="align-left data">
            <div v-if="Array.isArray(val)" class="few-items">
                <p v-for="str in val" :key="str" class="array-val">{{ str }}</p>
            </div>
            <p v-else>{{ val }}</p>
        </th>
        <slot name="options" />
    </tr>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VTableCheckbox from '@/components/common/VTableCheckbox.vue';

// @vue/component
@Component({
    components: { VTableCheckbox },
})
export default class TableItem extends Vue {
    @Prop({ default: false })
    public readonly selectDisabled: boolean;
    @Prop({ default: false })
    public readonly selected: boolean;
    @Prop({ default: false })
    public readonly selectable: boolean;
    @Prop({ default: () => {} })
    public readonly item: object;
    @Prop({ default: null })
    public readonly onClick: (data?: unknown) => void;

    public onChange(value: boolean): void {
        this.$emit('selectChange', value);
    }
}
</script>

<style scoped lang="scss">
    tr {

        &:hover {
            background-color: #e6e9ef;
        }

        &.selected {
            background: #f0f3f8;
        }
    }

    .few-items {
        display: flex;
        flex-direction: column;
        justify-content: space-between;
    }

    .array-val {
        font-family: 'font_regular', sans-serif;
        font-size: 0.75rem;
        line-height: 1.25rem;

        &:first-of-type {
            font-family: 'font_bold', sans-serif;
            font-size: 0.875rem;
            margin-bottom: 3px;
        }
    }
</style>
