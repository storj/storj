// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, ComputedRef, ref } from 'vue';

import { Validator } from '@/utils/validation';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';

export enum SortDirection {
    asc = 1,
    desc = 2,
}

export class PricingPlanInfo {
    public type: PricingPlanType = PricingPlanType.FREE;
    // Info for the pricing plan container
    public activationButtonText: string | null = null;
    // Info for the pricing plan modal (pre-activation)
    public activationSubtitle: string | null = null;
    // Info for the pricing plan modal (post-activation)
    public bannerTitle: string = '';
    public bannerText: string = '';
    // the following are used in the new upgrade/account setup
    // dialogs.
    public planTitle: string = '';
    public planSubtitle: string = '';
    public planCost: string = '';
    public planCostInfo: string = '';
    public planMinimumFeeInfo: string = '';
    public planUpfrontCharge: string = '';
    public planBalanceCredit: string = '';
    public planCTA: string = '';
    public planInfo: string[] = [];

    constructor(init?: Partial<PricingPlanInfo>) {
        Object.assign(this, init);
    }
}

export interface OnboardingInfo {
    accessText?: string;
    accessBtnText?: string;
    accessTitle?: string;
}

export enum PricingPlanType {
    FREE = 'free',
    PARTNER = 'partner',
    PRO = 'pro',
}

export const FREE_PLAN_INFO = new PricingPlanInfo({
    type: PricingPlanType.FREE,
    planTitle: 'Free Trial',
    planSubtitle: 'Perfect for trying out Storj.',
    planCost: 'Free',
    planCostInfo: '30 days trial, no card needed.',
    planCTA: 'Start Free Trial',
    planInfo: [
        '25GB storage included',
        '25GB download included',
        '1 project',
    ],
});

// TODO: fully implement these types and their methods according to their Go counterparts
export type UUID = string;
export type MemorySize = string;
export type Time = string;

export function tableSizeOptions(itemCount: number, isObjectBrowser = false): { title: string, value: number }[] {
    const opts = [
        { title: '10', value: 10 },
        { title: '25', value: 25 },
        { title: '50', value: 50 },
        { title: '100', value: 100 },
    ];
    if (itemCount <= 300 && !isObjectBrowser) {
        return [{ title: 'All', value: itemCount }, ...opts];
    }
    return opts;
}

export type DataTableHeader = {
    key: string;
    title: string;
    align?: 'start' | 'end' | 'center';
    sortable?: boolean;
    width?: number | string;
    maxWidth?: number | string;
};

export type SortItem = {
    key: string;
    order?: boolean | 'asc' | 'desc';
};

export type ValidationRule<T> = string | boolean | ((value: T) => string | boolean);

export function RequiredRule(value: unknown): string | boolean {
    return (Array.isArray(value) ? !!value.length : !!value) || 'Required';
}

export function EmailRule(value: string, strict = false): string | boolean {
    return Validator.email(value, strict) || 'E-mail must be valid.';
}

export function DomainRule(value: string): string | boolean {
    return Validator.domainName(value) || 'Domain must be valid.';
}

export function GoodPasswordRule(value: unknown): string | boolean {
    const badPasswords = useUsersStore().state.badPasswords;

    return badPasswords.has(value as string) ? 'Password is on the list of disallowed passwords.' : true;
}

export function MaxNameLengthRule(value: string): string | boolean {
    const { maxNameCharacters } = useConfigStore().state.config;

    return Validator.nameLength(value, maxNameCharacters) || `The value must be less than or equal to ${maxNameCharacters}.`;
}

export function PhoneNumberRule(value: string): string | boolean {
    return Validator.phoneNumber(value) || 'Phone number must be valid.';
}

export function PublicSSHKeyRule(value: string): string | boolean {
    return Validator.publicSSHKey(value) || 'SSH public key must be valid.';
}

export function HostnameRule(value: string): string | boolean {
    return Validator.hostname(value) || 'Hostname must be valid.';
}

export interface IDialogFlowStep {
    onEnter?: () => void;
    onExit?: (to: 'next' | 'prev') => void;
    validate?: () => boolean;
}

export interface DialogStepComponent extends IDialogFlowStep {
    title: string;
    iconSrc?: string;
}

export type SaveButtonsItem = string | {
    name: string;
    value: string;
};

export const MAX_SEARCH_VALUE_LENGTH = 200;

export function getUniqueName(name: string, allNames: string[]): string {
    // Regular expression to match a name with an optional numeric suffix.
    const namePattern = /^(.*?)(?: \((\d+)\))?$/;

    let currName = name;
    let count = 0;
    while (allNames.includes(currName)) {
        count++;
        currName = currName.replace(namePattern, (_, baseName, index) => {
            // Increment the suffix if it exists, or add a new one starting with 1.
            const newIndex = index ? parseInt(index) + 1 : count;
            return `${baseName} (${newIndex})`;
        });
    }

    return currName;
}

export type SetupLocation<T> = undefined | (() => (T | undefined));

export interface SetupStep {
    setup?: () => void | Promise<void>;
    validate?: () => boolean;
}

interface StepInfoData<T> {
    prev?: SetupLocation<T>,
    prevText?: string,
    next?: SetupLocation<T>,
    nextText?: string | (() => string),
    beforePrev?: () => void,
    beforeNext?: () => Promise<void>,
    setup?: () => void | Promise<void>,
    validate?: () => boolean,
    noRef?: boolean,
}

export class StepInfo<T> {
    public ref = ref<SetupStep>();
    public prev?: ComputedRef<T | undefined>;
    public next?: ComputedRef<T | undefined>;
    public prevText?: string;
    public nextText?: ComputedRef<string>;
    public beforePrev?: () => void;
    public beforeNext?: () => Promise<void>;
    public setup?: () => void | Promise<void>;
    public validate?: () => boolean;

    constructor(data: StepInfoData<T>) {
        if (!data.noRef) {
            this.ref = ref<SetupStep>();
        }
        this.prev = data.prev ? computed<T | undefined>(data.prev) : undefined;
        this.next = data.next ? computed<T | undefined>(data.next) : undefined;
        this.beforePrev = data.beforePrev;
        this.beforeNext = data.beforeNext;
        this.setup = data.setup;
        this.validate = data.validate;

        this.prevText = data.prevText ? data.prevText : (!data.prev) ? 'Cancel' : 'Back';

        this.nextText = computed(() => {
            if (typeof data.nextText === 'function') {
                return data.nextText();
            }
            return data.nextText ? data.nextText : (!data.next ? 'Done' : 'Next');
        });
    }
}

export interface StripeForm {
    onSubmit(): Promise<string>;
    initStripe(): Promise<string>;
}