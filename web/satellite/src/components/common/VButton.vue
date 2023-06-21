// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <!-- if isDisabled check onPress in parent element -->
    <a
        v-if="link"
        class="container"
        :href="link"
        :class="containerClassName"
        :style="style"
        tabindex="0"
        target="_blank"
        rel="noopener noreferrer"
        @click="onPress"
    >
        <div class="icon-wrapper">
            <slot name="icon" />
        </div>
        <span class="label" :class="{uppercase: isUppercase}">
            <component :is="iconComponent" v-if="iconComponent" />
            <span v-if="icon !== 'none'">&nbsp;&nbsp;</span>
            {{ label }}
        </span>
        <div class="icon-wrapper-right">
            <slot name="icon-right" />
        </div>
    </a>
    <div
        v-else
        class="container"
        :class="containerClassName"
        :style="style"
        tabindex="0"
        @click="handleClick"
        @keyup.enter="handleClick"
    >
        <div class="icon-wrapper">
            <slot name="icon" />
        </div>
        <span class="label" :class="{uppercase: isUppercase}">
            <component :is="iconComponent" v-if="iconComponent" />
            <span v-if="icon !== 'none'">&nbsp;&nbsp;</span>
            <slot />
            {{ label }}
        </span>
        <div class="icon-wrapper-right">
            <slot name="icon-right" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import WhitePlusIcon from '@/../static/images/common/plusWhite.svg';
import AddCircleIcon from '@/../static/images/common/addCircle.svg';
import CopyIcon from '@/../static/images/common/copyButtonIcon.svg';
import CheckIcon from '@/../static/images/common/check.svg';
import TrashIcon from '@/../static/images/accessGrants/trashIcon.svg';
import LockIcon from '@/../static/images/common/lockIcon.svg';
import CreditCardIcon from '@/../static/images/common/creditCardIcon-white.svg';
import DocumentIcon from '@/../static/images/common/documentIcon.svg';
import DownloadIcon from '@/../static/images/common/download.svg';
import FolderIcon from '@/../static/images/objects/newFolder.svg';
import ResourcesIcon from '@/../static/images/navigation/resources.svg';

const props = withDefaults(defineProps<{
    link?: string;
    label?: string;
    width?: string;
    height?: string;
    fontSize?: string;
    borderRadius?: string;
    icon?: string;
    isOrange?: boolean;
    isWhite?: boolean;
    isSolidDelete?: boolean;
    isTransparent?: boolean;
    isDeletion?: boolean;
    isGreyBlue?: boolean;
    isBlueWhite?: boolean;
    isWhiteGreen?: boolean;
    isGreen?: boolean;
    isDisabled?: boolean;
    isUppercase?: boolean;
    onPress?: () => void;
}>(), {
    link: undefined,
    label: '',
    width: 'inherit',
    height: 'inherit',
    fontSize: '16px',
    borderRadius: '6px',
    icon: 'none',
    isOrange: false,
    isWhite: false,
    isSolidDelete: false,
    isTransparent: false,
    isDeletion: false,
    isGreyBlue: false,
    isBlueWhite: false,
    isWhiteGreen: false,
    isGreen: false,
    isDisabled: false,
    isUppercase: false,
    onPress: () => {},
});

const icons = new Map<string, string>([
    ['copy', CopyIcon],
    ['check', CheckIcon],
    ['download', DownloadIcon],
    ['lock', LockIcon],
    ['credit-card', CreditCardIcon],
    ['document', DocumentIcon],
    ['trash', TrashIcon],
    ['folder', FolderIcon],
    ['resources', ResourcesIcon],
    ['addcircle', AddCircleIcon],
    ['add', WhitePlusIcon],
]);

const iconComponent = computed((): string | undefined => icons.get(props.icon.toLowerCase()));

const containerClassName = computed((): string => {
    if (props.isDisabled) return 'disabled';

    if (props.isWhite) return 'white';

    if (props.isOrange) return 'orange';

    if (props.isSolidDelete) return 'solid-red';

    if (props.isTransparent) return 'transparent';

    if (props.isDeletion) return 'red';

    if (props.isGreyBlue) return 'grey-blue';

    if (props.isBlueWhite) return 'blue-white';

    if (props.isWhiteGreen) return 'white-green';

    if (props.isGreen) return 'green';

    return '';
});

const style = computed(() => {
    return { width: props.width, height: props.height, borderRadius: props.borderRadius, fontSize: props.fontSize };
});

/**
 * This wrapper handles button's disabled state for accessibility purposes.
 */
function handleClick(): void {
    if (!props.isDisabled) {
        props.onPress();
    }
}
</script>

<style scoped lang="scss">
    .label {
        display: flex;
        align-items: center;
    }

    .transparent {
        background-color: transparent !important;
        border: 1px solid #afb7c1 !important;

        .label {
            color: #354049 !important;
        }

        :deep(path),
        :deep(rect) {
            fill: #354049 !important;
        }
    }

    .solid-red {
        background-color: var(--c-red-2) !important;
        border: 1px solid var(--c-red-2) !important;

        .label {
            color: #fff !important;
        }

        &:hover {
            background-color: var(--c-red-3) !important;
            border: 1px solid var(--c-red-3) !important;
        }
    }

    .white {
        background-color: #fff !important;
        border: 1px solid var(--c-grey-3) !important;

        .label {
            color: #354049 !important;
        }

        :deep(path),
        :deep(rect) {
            fill: #354049 !important;
        }
    }

    .blue-white {
        background-color: #fff !important;
        border: 2px solid #2683ff !important;

        .label {
            color: #2683ff !important;
        }
    }

    .white-green {
        background-color: transparent !important;
        border: 1px solid #d8dee3 !important;

        .label {
            color: var(--c-green-5) !important;
        }

        :deep(path),
        :deep(rect) {
            fill: var(--c-green-5) !important;
        }
    }

    .green {
        background-color: var(--c-green-5) !important;
    }

    .grey-blue {
        background-color: #fff !important;
        border: 2px solid #d9dbe9 !important;

        .label {
            color: var(--c-blue-3) !important;
        }
    }

    .disabled {
        background-color: var(--c-grey-5) !important;
        border-color: var(--c-grey-5) !important;
        pointer-events: none !important;

        .label {
            color: var(--c-white) !important;
        }
    }

    .red {
        background-color: #fff3f2 !important;
        border: 2px solid #e30011 !important;

        .label {
            color: #e30011 !important;
        }
    }

    .orange {
        background-color: #ff8a00 !important;
        border: 2px solid #ff8a00 !important;
    }

    .container {
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: var(--c-blue-3);
        cursor: pointer;
        box-sizing: border-box;

        :deep(path),
        :deep(rect) {
            fill: var(--c-white);
        }

        .trash-icon {
            margin-right: 5px;
        }

        .icon-wrapper {
            display: flex;

            &:not(:empty) {
                margin-right: 8px;
            }
        }

        .icon-wrapper-right {
            display: flex;

            &:not(:empty) {
                margin-left: 8px;
            }
        }

        .label {
            font-family: 'font_medium', sans-serif;
            color: #fff;
            margin: 0;
            white-space: nowrap;
        }

        &:hover {
            background-color: #0059d0;

            &.transparent,
            &.blue-white,
            &.white {
                box-shadow: none !important;
                background-color: #2683ff !important;
                border: 1px solid #2683ff !important;

                :deep(path),
                :deep(rect) {
                    fill: white !important;
                }

                .label {
                    color: white !important;
                }
            }

            &.grey-blue {
                background-color: #2683ff !important;
                border-color: #2683ff !important;

                .label {
                    color: white !important;
                }
            }

            &.blue-white {
                border: 2px solid #2683ff !important;
            }

            &.red {
                box-shadow: none !important;
                background-color: transparent !important;

                .label {
                    color: #eb5757 !important;
                }
            }

            &.orange {
                background-color: #c16900 !important;
                border-color: #c16900 !important;
            }

            &.white-green {
                background-color: var(--c-green-4) !important;
            }

            &.green {
                background-color: #008a1e !important;
            }

            &.disabled {
                box-shadow: none !important;
                background-color: #dadde5 !important;

                .label {
                    color: #acb0bc !important;
                }

                &:hover {
                    cursor: default;
                }
            }
        }
    }

    .uppercase {
        text-transform: uppercase;
    }
</style>
