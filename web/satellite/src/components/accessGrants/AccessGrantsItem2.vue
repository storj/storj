// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :item="itemToRender"
        :on-click="onClick"
    >
        <th slot="options" v-click-outside="closeDropdown" class="grant-item__functional options overflow-visible" @click.stop="openDropdown">
            <dots-icon />
            <div v-if="isDropdownOpen" class="grant-item__functional__dropdown">
                <div class="grant-item__functional__dropdown__item" @click.stop="onDeleteClick">
                    <delete-icon />
                    <p class="grant-item__functional__dropdown__item__label">Delete Access</p>
                </div>
            </div>
        </th>
    </table-item>
</template>

<script lang="ts">
import { Component, Prop } from 'vue-property-decorator';

import DeleteIcon from '../../../static/images/objects/delete.svg';
import DotsIcon from '../../../static/images/objects/dots.svg';

import { AccessGrant } from '@/types/accessGrants';

import Resizable from '@/components/common/Resizable.vue';
import TableItem from '@/components/common/TableItem.vue';

// @vue/component
@Component({
    components: {
        TableItem,
        DeleteIcon,
        DotsIcon,
    },
})
export default class AccessGrantsItem extends Resizable {
    @Prop({ default: new AccessGrant('', '', new Date(), '') })
    private readonly itemData: AccessGrant;
    @Prop({ default: () => () => {} })
    public readonly onClick: () => void;
    @Prop({ default: false })
    public readonly isDropdownOpen: boolean;
    @Prop({ default: -1 })
    public readonly dropdownKey: number;

    public get itemToRender(): { [key: string]: string | string[] } {
        if (!this.isMobile) return { name: this.itemData.name, date: this.itemData.localDate() };

        return { info: [ this.itemData.name, `Created ${this.itemData.localDate()}` ] };
    }

    /**
     * Closes dropdown.
     */
    public closeDropdown(): void {
        this.$emit('openDropdown', -1);
    }

    /**
     * Opens dropdown.
     */
    public openDropdown(): void {
        this.$emit('openDropdown', this.dropdownKey);
    }

    public async onDeleteClick(): Promise<void> {
        this.$emit('deleteClick', this.itemData);
        this.closeDropdown();
    }
}
</script>

<style scoped lang="scss">
    .grant-item {

        &__functional {
            padding: 0 10px;
            position: relative;
            cursor: pointer;

            &__dropdown {
                position: absolute;
                top: 25px;
                right: 15px;
                background: #fff;
                box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
                border-radius: 6px;
                width: 255px;
                padding: 10px 0;
                z-index: 100;

                &__item {
                    display: flex;
                    align-items: center;
                    padding: 20px 25px;
                    width: calc(100% - 50px);

                    &__label {
                        margin: 0 0 0 10px;
                    }

                    &:hover {
                        background-color: #f4f5f7;
                        font-family: 'font_medium', sans-serif;

                        svg :deep(path) {
                            fill: #0068dc;
                            stroke: #0068dc;
                        }
                    }
                }
            }
        }
    }
</style>
