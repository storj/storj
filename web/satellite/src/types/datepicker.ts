// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export class DateObj {
    constructor(public time: any = null) {}
}

export class Options {
    constructor(
        public type: string = 'multi-day',
        public isSundayFirst: boolean = false,
        public week: string[] = ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su'],
        public month: string[] = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'],
        public color = {
            checked: '#2683FF',
            header: '#2683FF',
            headerText: '#444C63',
        },
        public inputStyle = {
            'visibility': 'hidden',
            'width': '0',
        },
        public placeholder: string = '',
        public buttons = {
            ok: 'OK',
            cancel: 'Cancel',
        },
        public overlayOpacity: number = 0.5,
        public dismissible: boolean = true,
    ) {}
}
