// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all properties and methods present and available in the file/browser objects in Browser.
 */
export interface BrowserFile extends File {
  Key: string;
  LastModified: Date;
  Size: number;
  type: string;
}
