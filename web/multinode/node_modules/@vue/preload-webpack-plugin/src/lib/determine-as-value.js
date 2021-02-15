/**
 * @license
 * Copyright 2018 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

const assert = require('assert')
const path = require('path')
const { URL } = require('url')

function determineAsValue ({ optionsAs, href, file }) {
  assert(href, `The 'href' parameter was not provided.`)

  switch (typeof optionsAs) {
    case 'string': {
      return optionsAs
    }

    case 'function': {
      return optionsAs(href)
    }

    case 'undefined': {
      // If `as` value is not provided in option, dynamically determine the correct
      // value based on the suffix of filename.

      // We only care about the pathname, so just use any domain when constructing the URL.
      // Use file instead of href option because the publicPath part may be malformed.
      // See https://github.com/vuejs/vue-cli/issues/5672
      const url = new URL(file || href, 'https://example.com')
      const extension = path.extname(url.pathname)

      if (extension === '.css') {
        return 'style'
      }

      if (extension === '.woff2') {
        return 'font'
      }

      return 'script'
    }

    default:
      throw new Error(`The 'as' option isn't set to a recognized value: ${optionsAs}`)
  }
}

module.exports = determineAsValue
