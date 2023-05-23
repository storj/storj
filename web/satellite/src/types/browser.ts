// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import EmailIcon from '../../static/images/objects/email.svg';

export enum ShareOptions {
  Reddit = 'Reddit',
  Facebook = 'Facebook',
  Twitter = 'Twitter',
  HackerNews = 'Hacker News',
  LinkedIn = 'LinkedIn',
  Telegram = 'Telegram',
  WhatsApp = 'WhatsApp',
  Email = 'E-Mail',
}

export class ShareButtonConfig {
    constructor(
      public label: ShareOptions = ShareOptions.Email,
      public color: string = 'white',
      public link: string = '',
      public image: string = EmailIcon,
    ) {}
}
