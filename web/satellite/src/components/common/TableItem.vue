// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr @click="onClick">
        <th v-if="selectable" class="icon">
            <v-checkbox @setData="onChange" />
        </th>
        <th v-for="(val, key, index) in item" :key="index" class="align-left data">
            <p>{{ val }}</p>
        </th>
        <slot name="options" />
    </tr>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';
import VCheckbox from "@/components/common/VCheckbox.vue";

// @vue/component
@Component({
    components: { VCheckbox }
})
export default class TableItem extends Vue {
    @Prop({ default: false })
    public readonly selectable: boolean;
    @Prop({ default: () => {} })
    public readonly item: object;
    @Prop({ default: null })
    public readonly onClick: (data?: unknown) => void;

    public isSelected = false;

    public onChange(value: boolean): void {
        this.isSelected = value;
        this.$emit('setSelected', value);
    }
}
</script>
