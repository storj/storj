// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="tag" :class="{member: !isOwner}">
        <box-icon v-if="!noIcon" class="tag__icon" />

        <span class="tag__text"> {{ isOwner ? 'Owner': 'Member' }} </span>
    </div>
</template>

<script setup lang="ts">
import BoxIcon from '@/../static/images/allDashboard/box.svg';

const props = withDefaults(defineProps<{
    isOwner: boolean,
    noIcon?: boolean,
}>(), {
    isOwner: false,
    noIcon: false,
});

</script>

<style scoped lang="scss">
.tag {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 5px;
    padding: 4px 8px;
    border: 1px solid var(--c-purple-2);
    border-radius: 24px;
    color: var(--c-purple-4);

    &__text {
        font-size: 12px;
        font-family: 'font_regular', sans-serif;
    }

    &.member {
        color: var(--c-yellow-5);
        border-color: var(--c-yellow-2);

        :deep(path) {
            fill: var(--c-yellow-5);
        }
    }
}
</style>
