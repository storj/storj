// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <!-- if isDisabled check onPress in parent element -->
    <div
        :class="containerClassName"
        :style="style"
        tabindex="0"
        @click="onPress"
        @keyup.enter="onPress"
    >
        <slot name="icon" />
        <div v-if="isWhiteGreen" class="greenCheck">&#x2713;</div>
        <div v-if="isGreenWhite" class="whiteCheck">&#x2713;</div>
        <span class="label" :class="{uppercase: isUppercase}">
            <CopyIcon v-if="icon.toLowerCase() === 'copy'" />
            <LockIcon v-if="icon.toLowerCase() === 'lock'" />
            <CreditCardIcon v-if="icon.toLowerCase() === 'credit-card'" />
            <DocumentIcon v-if="icon.toLowerCase() === 'document'" />
            <TrashIcon v-if="icon.toLowerCase() === 'trash'" />
            <FolderIcon v-if="icon.toLowerCase() === 'folder'" />
            <span v-if="icon !== 'none'">&nbsp;&nbsp;</span>{{ label }}</span>
    </div>
</template>

<script setup lang="ts">

import { computed } from 'vue';

import CopyIcon from '@/../static/images/common/copyButtonIcon.svg';
import TrashIcon from '@/../static/images/accessGrants/trashIcon.svg';
import LockIcon from '@/../static/images/common/lockIcon.svg';
import CreditCardIcon from '@/../static/images/common/creditCardIcon-white.svg';
import DocumentIcon from '@/../static/images/common/documentIcon.svg';
import FolderIcon from '@/../static/images/objects/newFolder.svg';

const props = withDefaults(defineProps<{
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
    isGreenWhite?: boolean;
    isDisabled?: boolean;
    isUppercase?: boolean;
    onPress?: () => void;
}>(), {
    label: 'Default',
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
    isGreenWhite: false,
    isDisabled: false,
    isUppercase: false,
    onPress: () => {},
});

const containerClassName = computed((): string => {
    if (props.isDisabled) return 'container disabled';

    if (props.isWhite) return 'container white';

    if (props.isOrange) return 'container orange';

    if (props.isSolidDelete) return 'container solid-red';

    if (props.isTransparent) return 'container transparent';

    if (props.isDeletion) return 'container red';

    if (props.isGreyBlue) return 'container grey-blue';

    if (props.isBlueWhite) return 'container blue-white';

    if (props.isWhiteGreen) return 'container white-green';

    if (props.isGreenWhite) return 'container green-white';

    return 'container';
});

const style = computed(() => {
    return { width: props.width, height: props.height, borderRadius: props.borderRadius, fontSize: props.fontSize };
});
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
    }

    .solid-red {
        background-color: var(--c-red-3) !important;
        border: 1px solid var(--c-red-3) !important;

        .label {
            color: #fff !important;
        }

        &:hover {
            background-color: #790000 !important;
            border: 1px solid #790000 !important;
        }
    }

    .white {
        background-color: #fff !important;
        border: 1px solid var(--c-grey-3) !important;

        .label {
            color: #354049 !important;
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
        border: 1px solid #afb7c1 !important;

        .label {
            color: var(--c-green-5) !important;
        }
    }

    .green-white {
        background-color: var(--c-green-5) !important;
        border: 1px solid var(--c-green-5) !important;
    }

    .grey-blue {
        background-color: #fff !important;
        border: 2px solid #d9dbe9 !important;

        .label {
            color: var(--c-blue-3) !important;
        }
    }

    .disabled {
        background-color: #dadde5 !important;
        border-color: #dadde5 !important;
        pointer-events: none !important;

        .label {
            color: #acb0bc !important;
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

        .trash-icon {
            margin-right: 5px;
        }

        .greenCheck {
            color: var(--c-green-5) !important;
            margin-right: 5px;
        }

        .whiteCheck {
            color: #fff !important;
            margin-right: 5px;
        }

        .label {
            font-family: 'font_medium', sans-serif;
            line-height: 23px;
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
                    stroke: white;
                    fill: white;
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
