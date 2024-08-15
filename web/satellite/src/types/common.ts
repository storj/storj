// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { Validator } from '@/utils/validation';
import { useConfigStore } from '@/store/modules/configStore';

export enum SortDirection {
    asc = 1,
    desc = 2,
}

export class PricingPlanInfo {
    constructor(
        public type: PricingPlanType = PricingPlanType.FREE,
        // Info for the pricing plan container
        public title: string = '',
        public containerSubtitle: string = '',
        public containerDescription: string = '',
        public containerFooterHTML: string | null = null,
        public activationButtonText: string | null = null,
        // Info for the pricing plan modal (pre-activation)
        public activationSubtitle: string | null = null,
        public activationDescriptionHTML: string = '',
        public activationPriceHTML: string | null = null,
        // Info for the pricing plan modal (post-activation)
        public successSubtitle: string = '',
        public bannerTitle: string = '',
        public bannerText: string = '',
    ) {}
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

export const PRO_PLAN_INFO = new PricingPlanInfo(
    PricingPlanType.PRO,
    'Pro Account',
    'Pay-as-you-go, no minimum',
    'Pay for what you need. $4/TB storage per month, $7/TB for download bandwidth.',
    'Additional per-segment fee of $0.0000088 applies.',
    null,
    null,
    'Add a credit card to activate your pro account. Only pay for what you use, no minimum. Billed monthly.',
    'No charge today',
    '',
);

export const FREE_PLAN_INFO = new PricingPlanInfo(
    PricingPlanType.FREE,
    'Free Trial',
    'Limited 30-day trial',
    'Try Storj for free with 25GB of storage and 25GB download bandwidth for 30 days.',
    'Upgrade anytime to Pro account to continue using Storj.',
    null,
    null,
    'Start for free to try Storj and upgrade later.',
    null,
    'Limited 25',
);

// TODO: fully implement these types and their methods according to their Go counterparts
export type UUID = string
export type MemorySize = string
export type Time = string

export function tableSizeOptions(itemCount: number, isObjectBrowser = false): {title: string, value: number}[] {
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

export function MaxNameLengthRule(value: string): string | boolean {
    const { maxNameCharacters } = useConfigStore().state.config;

    return Validator.nameLength(value, maxNameCharacters) || `The value must be less than or equal to ${maxNameCharacters}.`;
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
