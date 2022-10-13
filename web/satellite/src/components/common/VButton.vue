// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <!-- if isDisabled check onPress in parent element -->
    <div
        :class="containerClassName"
        :style="style"
        @click="onPress"
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
            <span v-if="icon !== 'none'">&nbsp;&nbsp;</span>{{ label }}</span>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import CopyIcon from '@/../static/images/common/copyButtonIcon.svg';
import TrashIcon from '@/../static/images/accessGrants/trashIcon.svg';
import LockIcon from '@/../static/images/common/lockIcon.svg';
import CreditCardIcon from '@/../static/images/common/creditCardIcon-white.svg';
import DocumentIcon from '@/../static/images/common/documentIcon.svg';

/**
 * Custom button component with label.
 */
// @vue/component
@Component({
    components: {
        CopyIcon,
        LockIcon,
        CreditCardIcon,
        DocumentIcon,
        TrashIcon,
    },
})
export default class VButton extends Vue {
    @Prop({ default: 'Default' })
    private readonly label: string;
    @Prop({ default: 'inherit' })
    private readonly width: string;
    @Prop({ default: 'inherit' })
    private readonly height: string;
    @Prop({ default: '16px' })
    private readonly fontSize: string;
    @Prop({ default: '6px' })
    private readonly borderRadius: string;
    @Prop({ default: 'none' })
    private readonly icon: string;
    @Prop({ default: false })
    private readonly isWhite: boolean;
    @Prop({ default: false })
    private readonly isSolidDelete: boolean;
    @Prop({ default: false })
    private readonly isTransparent: boolean;
    @Prop({ default: false })
    private readonly isDeletion: boolean;
    @Prop({ default: false })
    private readonly isGreyBlue: boolean;
    @Prop({ default: false })
    private readonly isBlueWhite: boolean;
    @Prop({ default: false })
    private readonly isWhiteGreen: boolean;
    @Prop({ default: false })
    private readonly isGreenWhite: boolean;
    @Prop({ default: false })
    private isDisabled: boolean;
    @Prop({ default: false })
    private readonly isUppercase: boolean;
    @Prop({ default: () => () => {} })
    private readonly onPress: () => void;

    public get style(): Record<string, unknown> {
        return { width: this.width, height: this.height, borderRadius: this.borderRadius, fontSize: this.fontSize };
    }

    public get containerClassName(): string {
        if (this.isDisabled) return 'container disabled';

        if (this.isWhite) return 'container white';

        if (this.isSolidDelete) return 'container solid-red';

        if (this.isTransparent) return 'container transparent';

        if (this.isDeletion) return 'container red';

        if (this.isGreyBlue) return 'container grey-blue';

        if (this.isBlueWhite) return 'container blue-white';

        if (this.isWhiteGreen) return 'container white-green';

        if (this.isGreenWhite) return 'container green-white';

        return 'container';
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
    }

    .solid-red {
        background-color: #ba0000 !important;
        border: 1px solid #ba0000 !important;

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
        border: 1px solid #d8dee3 !important;

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
            color: #00ac26 !important;
        }
    }

    .green-white {
        background-color: #00ac26 !important;
        border: 1px solid #00ac26 !important;
    }

    .grey-blue {
        background-color: #fff !important;
        border: 2px solid #d9dbe9 !important;

        .label {
            color: #0149ff !important;
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

    .container {
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: #0149ff;
        cursor: pointer;

        .trash-icon {
            margin-right: 5px;
        }

        .greenCheck {
            color: #00ac26 !important;
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
