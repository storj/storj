// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="tag" :class="{[role.toLowerCase().replaceAll(' ', '-')]: true}">
        <component :is="icon" v-if="!noIcon" class="tag__icon" />
        <span class="tag__text">{{ role }}</span>
    </div>
</template>

<script setup lang="ts">
import { computed, Component } from 'vue';

import { ProjectRole } from '@/types/projectMembers';

import BoxIcon from '@/../static/images/navigation/project.svg';
import InviteIcon from '@/../static/images/navigation/quickStart.svg';
import ClockIcon from '@/../static/images/team/clock.svg';

const props = withDefaults(defineProps<{
    role: ProjectRole,
    noIcon?: boolean,
}>(), {
    role: ProjectRole.Member,
    noIcon: false,
});

const icon = computed((): string => {
    switch (props.role) {
    case ProjectRole.Invited:
        return InviteIcon;
    case ProjectRole.InviteExpired:
        return ClockIcon;
    default:
        return BoxIcon;
    }
});

</script>

<style scoped lang="scss">
.tag {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 5px;
    padding: 4px 8px;
    border: 1px solid var(--c-yellow-2);
    border-radius: 24px;
    color: var(--c-yellow-5);
    background: var(--c-white);

    :deep(path) {
        fill: var(--c-yellow-5);
    }

    &__icon {
        width: 12px;
        height: 12px;
    }

    &__text {
        font-size: 12px;
        font-family: 'font_regular', sans-serif;
    }

    &.owner {
        color: var(--c-purple-4);
        border-color: var(--c-purple-2);

        :deep(path) {
            fill: var(--c-purple-4);
        }
    }

    &.invited,
    &.invite-expired {
        color: var(--c-grey-6);
        border-color: var(--c-grey-4);
    }

    &.invited :deep(path) {
        fill: var(--c-yellow-3);
    }

    &.invite-expired :deep(path) {
        fill: var(--c-grey-4);
    }
}
</style>
