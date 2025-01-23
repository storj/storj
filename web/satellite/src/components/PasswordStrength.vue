// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="password-strength mb-5 rounded-lg border" role="group" aria-label="Password strength indicator">
        <!-- Simple strength indicator -->
        <div class="d-flex align-center justify-space-between">
            <div class="d-flex align-center">
                <span
                    class="text-body-2 font-weight-bold"
                    :style="strengthLabelColor"
                    aria-live="polite"
                >
                    {{ props.password.length >= passMinLength ? 'Password strength: ' : '' }}{{ passwordStrength }}
                </span>
                <v-icon
                    v-if="passwordStrength === PASSWORD_STRENGTH.strong || passwordStrength === PASSWORD_STRENGTH.veryStrong" class="ml-1"
                    color="success"
                    size="16"
                >
                    <Check stroke-width="3" />
                </v-icon>
            </div>
            <v-btn
                icon
                size="xx-small"
                variant="text"
                color="default"
                rounded="md"
                class="transition-transform px-0"
                :class="{ 'rotate-180': expanded }"
                :aria-expanded="expanded"
                aria-controls="password-requirements"
                title="Show password requirements"
                @click="expanded = !expanded"
            >
                <v-icon size="16">
                    <ChevronDown />
                </v-icon>
            </v-btn>
        </div>
        <v-progress-linear
            :model-value="barWidth"
            :color="passwordStrengthColor"
            rounded="lg"
            class="mt-1 mb-4"
            height="3"
            role="progressbar"
        />

        <!-- Expandable details -->
        <v-expand-transition>
            <v-card
                v-if="expanded"
                id="password-requirements"
                class="mb-4"
                width="100%"
                rounded="md"
                role="region"
                aria-label="Password requirements"
            >
                <v-card-item>
                    <p class="text-body-2 font-weight-medium mt-1">Your password should contain:</p>
                    <v-checkbox
                        tabindex="-1"
                        class="no-pointer-events text-body-2"
                        :model-value="isPasswordLengthAcceptable"
                        :aria-checked="isPasswordLengthAcceptable"
                        color="success"
                        density="compact"
                        hide-details
                    >
                        <template #label>
                            <p class="text-body-2">Between {{ passMinLength }} and {{ passMaxLength }} Latin characters</p>
                        </template>
                    </v-checkbox>

                    <p class="text-body-2 font-weight-medium">Its nice to have:</p>
                    <v-checkbox
                        tabindex="-1"
                        class="no-pointer-events text-body-2"
                        :model-value="hasLowerAndUpperCaseLetters"
                        color="success"
                        density="compact"
                        hide-details
                    >
                        <template #label>
                            <p class="text-body-2">Upper and lowercase letters</p>
                        </template>
                    </v-checkbox>
                    <v-checkbox
                        tabindex="-1"
                        class="no-pointer-events text-body-2 mt-n3"
                        :model-value="hasSpecialCharacter"
                        color="success"
                        density="compact"
                        hide-details
                    >
                        <template #label>
                            <p class="text-body-2">At least one special character</p>
                        </template>
                    </v-checkbox>

                    <p class="text-caption text-high-emphasis">
                        Avoid using a password that you use on other websites or that might be easily guessed by someone else.
                    </p>
                </v-card-item>
            </v-card>
        </v-expand-transition>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
    VCard,
    VCardItem,
    VProgressLinear,
    VCheckbox,
    VBtn,
    VIcon,
    VExpandTransition,
} from 'vuetify/components';
import { Check, ChevronDown } from 'lucide-vue-next';

import { useConfigStore } from '@/store/modules/configStore';

const configStore = useConfigStore();
const expanded = ref(false);

const PASSWORD_STRENGTH = {
    veryStrong: 'Very Strong',
    strong: 'Strong',
    good: 'Good',
    weak: 'Weak',
};

const PASSWORD_STRENGTH_COLORS = {
    [PASSWORD_STRENGTH.good]: '#ffa500',
    [PASSWORD_STRENGTH.strong]: '#00AC26',
    [PASSWORD_STRENGTH.veryStrong]: '#00AC26',
    default: '#ff0000',
};

const BAR_WIDTH = {
    [PASSWORD_STRENGTH.weak]: '25',
    [PASSWORD_STRENGTH.good]: '50',
    [PASSWORD_STRENGTH.strong]: '75',
    [PASSWORD_STRENGTH.veryStrong]: '100',
    default: '0',
};

const props = defineProps<{
    password: string;
    email?: string;
}>();

const passMaxLength = computed((): number => {
    return configStore.state.config.passwordMaximumLength;
});

const passMinLength = computed((): number => {
    return configStore.state.config.passwordMinimumLength;
});

const isPasswordLengthAcceptable = computed((): boolean => {
    return props.password.length <= passMaxLength.value
        && props.password.length >= passMinLength.value;
});

const passwordStrength = computed((): string => {
    if (props.password.length < passMinLength.value) {
        return `Minimum ${passMinLength.value} characters`;
    }

    if (props.password.length > passMaxLength.value) {
        return `Maximum ${passMaxLength.value} characters`;
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

const passwordStrengthColor = computed((): string => {
    return PASSWORD_STRENGTH_COLORS[passwordStrength.value] || PASSWORD_STRENGTH_COLORS.default;
});

const barWidth = computed((): string => {
    return BAR_WIDTH[passwordStrength.value] || BAR_WIDTH.default;
});

const strengthLabelColor = computed((): { color: string } => {
    return { color: passwordStrengthColor.value };
});

const hasLowerAndUpperCaseLetters = computed((): boolean => {
    return /[a-z]/.test(props.password) && /[A-Z]/.test(props.password);
});

const hasSpecialCharacter = computed((): boolean => {
    return /\W/.test(props.password);
});

function scorePassword(): number {
    const password = props.password;
    let score = 0;

    const letters: number[] = [];
    for (let i = 0; i < password.length; i++) {
        letters[password[i]] = (letters[password[i]] || 0) + 1;
        score += 5 / letters[password[i]];
    }
    score = Math.min(score, 60);

    const variations: boolean[] = [
        /\d/.test(password),
        /[a-z]/.test(password),
        /[A-Z]/.test(password),
        /\W/.test(password),
    ];

    score += variations.filter(Boolean).length * 10;

    const isSequential = (str: string): boolean => {
        const sequences = 'abcdefghijklmnopqrstuvwxyz0123456789';
        const reversed = sequences.split('').reverse().join('');

        for (let i = 0; i < str.length - 2; i++) {
            const substr = str.slice(i, i + 3);
            if (sequences.includes(substr) || reversed.includes(substr)) {
                return true;
            }
        }
        return false;
    };

    if (isSequential(password.toLowerCase())) {
        // Penalize sequential patterns
        score -= 20;
    }

    if (props.email && password === props.email) {
        // Penalize password that is the same as email
        score -= 20;
    }

    return Math.max(score, 0);
}
</script>

<style scoped lang="scss">
.password-strength {
    width: 100%;
    background: rgb(var(--v-theme-background));
    padding: 12px 16px 2px;

    // border: 1px solid rgb(var(--v-theme-border));

    .v-progress-linear {
        transition: all 0.3s ease;
    }
}

.strength-label-transition {
    transition: color 0.3s ease;
}

.requirements-card-enter-active,
.requirements-card-leave-active {
    transition: opacity 0.3s, transform 0.3s;
}

.requirements-card-enter-from,
.requirements-card-leave-to {
    opacity: 0;
    transform: translateY(-10px);
}

.transition-transform {
    transition: transform 0.3s ease;
}

.rotate-180 {
    transform: rotate(180deg);
}
</style>