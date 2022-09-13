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
            <p class="password-strength-container__rule-area__rule">Between 6 and 128 Latin characters</p>
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

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { Validator } from '@/utils/validation';

import VectorIcon from '@/../static/images/register/StrengthVector.svg';

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

/**
 * StrengthLabelColor class holds info for StrengthLabelColor entity.
 */
class StrengthLabelColor {
    color: string;

    public constructor(color: string) {
        this.color = color;
    }
}

// @vue/component
@Component({
    components: {
        VectorIcon,
    },
})
export default class PasswordStrength extends Vue {
    @Prop({ default: '' })
    private readonly passwordString: string;
    /**
     * Indicates if component should be rendered.
     */
    @Prop({ default: false })
    private readonly isShown: boolean;

    public get isPasswordLengthAcceptable(): boolean {
        return Validator.password(this.passwordString);
    }

    /**
     * Returns password strength label depends on score.
     */
    public get passwordStrength(): string {
        if (this.passwordString.length < Validator.PASS_MIN_LENGTH) {
            return `Use ${Validator.PASS_MIN_LENGTH} or more characters`;
        }

        if (this.passwordString.length > Validator.PASS_MAX_LENGTH) {
            return `Use ${Validator.PASS_MAX_LENGTH} or fewer characters`;
        }

        const score = this.scorePassword();
        if (score > 90) {
            return 'Very Strong';
        }
        if (score > 70) {
            return 'Strong';
        }
        if (score > 45) {
            return 'Good';
        }

        return 'Weak';
    }

    public get barFillStyle(): BarFillStyle {
        return new BarFillStyle(this.passwordStrengthColor, this.barWidth);
    }

    public get strengthLabelColor(): StrengthLabelColor {
        return new StrengthLabelColor(this.passwordStrengthColor);
    }

    public get hasLowerAndUpperCaseLetters(): boolean {
        return /[a-z]/.test(this.passwordString) && /[A-Z]/.test(this.passwordString);
    }

    public get hasSpecialCharacter(): boolean {
        return /\W/.test(this.passwordString);
    }

    /**
     * Returns password strength score depends on length, case variations and special characters.
     */
    private scorePassword(): number {
        const password: string = this.passwordString;
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

    /**
     * Color for indicator between red as weak and green as strong password.
     */
    private get passwordStrengthColor(): string {
        switch (this.passwordStrength) {
        case 'Good':
            return '#ffff00';
        case 'Strong':
            return '#bfff00';
        case 'Very Strong':
            return '#00ff40';
        }

        return '#e16c58';
    }

    /**
     * Fills password strength indicator bar.
     */
    private get barWidth(): string {
        switch (this.passwordStrength) {
        case 'Weak':
            return '25%';
        case 'Good':
            return '50%';
        case 'Strong':
            return '75%';
        case 'Very Strong':
            return '100%';
        }

        return '0px';
    }
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
