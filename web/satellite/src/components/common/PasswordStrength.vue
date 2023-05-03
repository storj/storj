// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="isShown" class="password-strength-container">
        <div class="password-strength-container__header">
            <p class="password-strength-container__header__title">Password strength</p>
            <p class="password-strength-container__header__strength-status" :style="strengthLabelColor">{{ passwordStrength }}</p>
        </div>
        <div class="password-strength-container__bar">
            <div class="password-strength-container__bar__fill" :style="barFillStyle" />
        </div>
        <p class="password-strength-container__subtitle">Your password should contain:</p>
        <div class="password-strength-container__rule-area">
            <div class="password-strength-container__rule-area__checkbox" :class="{ checked: isPasswordLengthAcceptable }">
                <VectorIcon />
            </div>
            <p class="password-strength-container__rule-area__rule">Between {{ passMinLength }} and {{ passMaxLength }} Latin characters</p>
        </div>
        <p class="password-strength-container__subtitle">Its nice to have: </p>
        <div class="password-strength-container__rule-area">
            <div class="password-strength-container__rule-area__checkbox" :class="{ checked: hasLowerAndUpperCaseLetters }">
                <VectorIcon />
            </div>
            <p class="password-strength-container__rule-area__rule">Upper & lowercase letters</p>
        </div>
        <div class="password-strength-container__rule-area">
            <div class="password-strength-container__rule-area__checkbox" :class="{ checked: hasSpecialCharacter }">
                <VectorIcon />
            </div>
            <p class="password-strength-container__rule-area__rule">At least one special character</p>
        </div>
        <p class="password-strength-container__subtitle">Avoid using a password that you use on other websites or that might be easily guessed by someone else.</p>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { useConfigStore } from '@/store/modules/configStore';

import VectorIcon from '@/../static/images/register/StrengthVector.svg';

const configStore = useConfigStore();

/**
 * BarFillStyle class holds info for BarFillStyle entity.
 */
class BarFillStyle {
    'background-color': string;
    width: string;

    public constructor(backgroundColor: string, width: string) {
        this['background-color'] = backgroundColor;
        this.width = width;
    }
}

const PASSWORD_STRENGTH = {
    veryStrong: 'Very Strong',
    strong: 'Strong',
    good: 'Good',
    weak: 'Weak',
};

const PASSWORD_STRENGTH_COLORS = {
    [PASSWORD_STRENGTH.good]: '#ffff00',
    [PASSWORD_STRENGTH.strong]: '#bfff00',
    [PASSWORD_STRENGTH.veryStrong]: '#00ff40',
    default: '#e16c58',
};

const BAR_WIDTH = {
    [PASSWORD_STRENGTH.weak]: '25%',
    [PASSWORD_STRENGTH.good]: '50%',
    [PASSWORD_STRENGTH.strong]: '75%',
    [PASSWORD_STRENGTH.veryStrong]: '100%',
    default: '0px',
};

const props = withDefaults(defineProps<{
    passwordString?: string;
    isShown?: boolean;
}>(), {
    passwordString: '',
    isShown: false,
});

/**
 * Returns the maximum password length from the store.
 */
const passMaxLength = computed((): number => {
    return configStore.state.config.passwordMaximumLength;
});

/**
 * Returns the minimum password length from the store.
 */
const passMinLength = computed((): number => {
    return configStore.state.config.passwordMinimumLength;
});

const isPasswordLengthAcceptable = computed((): boolean => {
    return props.passwordString.length <= passMaxLength.value
        && props.passwordString.length >= passMinLength.value;
});

/**
 * Returns password strength label depends on score.
 */
const passwordStrength = computed((): string => {
    if (props.passwordString.length < passMinLength.value) {
        return `Use ${passMinLength.value} or more characters`;
    }

    if (props.passwordString.length > passMaxLength.value) {
        return `Use ${passMaxLength.value} or fewer characters`;
    }

    const score = scorePassword();
    if (score > 90) {
        return PASSWORD_STRENGTH.veryStrong;
    }
    if (score > 70) {
        return PASSWORD_STRENGTH.strong;
    }
    if (score > 45) {
        return PASSWORD_STRENGTH.good;
    }

    return PASSWORD_STRENGTH.weak;
});

/**
 * Color for indicator between red as weak and green as strong password.
 */
const passwordStrengthColor = computed((): string => {
    return PASSWORD_STRENGTH_COLORS[passwordStrength.value] || PASSWORD_STRENGTH_COLORS.default;
});

/**
 * Fills password strength indicator bar.
 */
const barWidth = computed((): string => {
    return BAR_WIDTH[passwordStrength.value] || BAR_WIDTH.default;
});

const strengthLabelColor = computed((): { color: string } => {
    return { color: passwordStrengthColor.value };
});

const hasLowerAndUpperCaseLetters = computed((): boolean => {
    return /[a-z]/.test(props.passwordString) && /[A-Z]/.test(props.passwordString);
});

const hasSpecialCharacter = computed((): boolean => {
    return /\W/.test(props.passwordString);
});

const barFillStyle = computed((): BarFillStyle => {
    return new BarFillStyle(passwordStrengthColor.value, barWidth.value);
});

/**
 * Returns password strength score depends on length, case variations and special characters.
 */
function scorePassword(): number {
    const password: string = props.passwordString;
    let score = 0;

    const letters: number[] = [];
    for (let i = 0; i < password.length; i++) {
        letters[password[i]] = (letters[password[i]] || 0) + 1;
        score += 5 / letters[password[i]];
    }

    const variations: boolean[] = [
        /\d/.test(password),
        /[a-z]/.test(password),
        /[A-Z]/.test(password),
        /\W/.test(password),
    ];

    let variationCount = 0;
    variations.forEach((check) => {
        variationCount += check ? 1 : 0;
    });

    score += variationCount * 10;

    return score;
}
</script>

<style scoped lang="scss">
    p {
        margin: 0;
    }

    .password-strength-container {
        position: absolute;
        top: 96px;
        right: -3px;
        padding: 25px 20px;
        border: 1px solid rgb(193 193 193 / 30%);
        box-shadow: 0 4px 20px rgb(204 208 214 / 25%);
        border-radius: 6px;
        background-color: #fff;
        height: 220px;
        width: 325px;
        z-index: 100;
        font-family: 'font_medium', sans-serif;

        &__header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            font-size: 14px;
            line-height: 19px;

            &__title {
                color: #384b65;
            }
        }

        &__bar {
            height: 3px;
            width: 100%;
            border-radius: 17px;
            background-color: #afb7c1;
            margin: 5px 0 0;
            position: relative;

            &__fill {
                height: 100%;
                position: absolute;
                left: 0;
                top: 0;
                border-radius: 17px;
            }
        }

        &__subtitle {
            font-size: 12px;
            line-height: 16px;
            color: #afb7c1;
            margin: 10px 0 0;
            text-align: left;
        }

        &__rule-area {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            margin: 10px 0 0;

            &__checkbox {
                height: 20px;
                width: 20px;
                border-radius: 10px;
                border: 1.5px solid #737791;
                display: flex;
                align-items: center;
                justify-content: center;
            }

            &__rule {
                font-size: 12px;
                line-height: 16px;
                color: #384b65;
                margin: 0 0 0 5px;
            }
        }
    }

    .checked {
        background-color: #27ae60;
        border-color: #27ae60;
    }
</style>
