// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="grants-item-container">
        <div class="grants-item-container__common-info">
            <div class="name-container" :title="itemData.name">
                <p class="name">{{ itemData.name }}</p>
            </div>
        </div>
        <div class="grants-item-container__common-info date-item-container">
            <p class="date">{{ itemData.localDate() }}</p>
        </div>

        <div class="grants-item-container__common-info menu-item-container">
            <p class="ellipses" @click.stop="togglePopupVisibility">...</p>
            <div
                v-if="popupVisible"
                v-click-outside="togglePopupVisibility"
                class="popup-menu"
            >
                <p
                    class="popup-menu__popup-delete"
                    @mouseenter="isPopupHovered = true"
                    @mouseleave="isPopupHovered = false"
                    @click="toggleSelection"
                >
                    <TrashIconWhite
                        v-if="isPopupHovered"
                    />
                    <TrashIconBlack
                        v-if="!isPopupHovered"
                    />
                    Delete Access
                </p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';
import TrashIconWhite from '@/../static/images/accessGrants/trashIcon.svg';
import TrashIconBlack from '@/../static/images/accessGrants/trashIcon-black.svg';
import { AccessGrant } from '@/types/accessGrants';

import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';

const {
    TOGGLE_SELECTION,
} = ACCESS_GRANTS_ACTIONS;

// @vue/component
@Component({
    components: {
        TrashIconWhite,
        TrashIconBlack,
    },
})
export default class AccessGrantsItem extends Vue {
    @Prop({ default: new AccessGrant('', '', new Date(), '') })
    private readonly itemData: AccessGrant;
    private popupVisible = false;
    private isPopupHovered = false;

    public togglePopupVisibility(): void {
        this.popupVisible = !this.popupVisible;
    }

    /**
     * Toggles access grant selection.
     */
    public async toggleSelection(): Promise<void> {
        await this.$store.dispatch(TOGGLE_SELECTION, this.itemData);
        this.$emit('altMethod');
        this.togglePopupVisibility();
        this.isPopupHovered = false;

    }
}
</script>

<style scoped lang="scss">
    @mixin popup-menu-button {
        padding: 0 15px;
        height: 100%;
        line-height: 50px;
        text-align: center;
        font-family: 'font_regular', sans-serif;
        color: #1b2533;
        transition: 100ms;
    }

    .grants-item-container {
        display: flex;
        align-items: center;
        justify-content: flex-start;
        height: 83px;
        background-color: #fff;
        border: 1px solid #e5e7eb;
        border-bottom: 0;
        width: 100%;

        &__common-info {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            width: 60%;
        }
    }

    .name-container {
        max-width: calc(100% - 131px);
        margin-right: 15px;
        min-width: 100px;
    }

    .name {
        font-family: 'font_bold', sans-serif;
        font-size: 16px;
        line-height: 21px;
        color: #354049;
        margin-left: 38px;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .date {
        font-family: 'font_regular', sans-serif;
        font-size: 16px;
        line-height: 21px;
        color: #354049;
        margin: 0;
    }

    .ellipses {
        margin: 0 auto 20px;
        font-size: 30px;
        font-weight: 1000;
        color: #7c8794;
        cursor: pointer;
    }

    .popup-menu {
        width: 160px;
        height: 50px;
        position: absolute;
        right: 40%;
        bottom: -65%;
        z-index: 1;
        background: #fff;
        border-radius: 10px;
        box-shadow: 0 20px 34px rgb(10 27 44 / 28%);

        &__popup-delete {
            @include popup-menu-button;

            border-radius: 10px;

            &:hover {
                background-color: #b53737;
                cursor: pointer;
                color: #fff;
            }
        }
    }

    .date-item-container {
        width: 50%;
    }

    .menu-item-container {
        width: 10%;
        position: relative;
    }
</style>
