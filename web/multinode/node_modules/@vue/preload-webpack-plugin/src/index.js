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

const defaultOptions = require('./lib/default-options')
const determineAsValue = require('./lib/determine-as-value')
const doesChunkBelongToHTML = require('./lib/does-chunk-belong-to-html')
const extractChunks = require('./lib/extract-chunks')

class PreloadPlugin {
  constructor (options) {
    this.options = Object.assign({}, defaultOptions, options)
  }

  generateLinks (compilation, htmlPluginData) {
    const options = this.options
    const extractedChunks = extractChunks({
      compilation,
      optionsInclude: options.include
    })

    const htmlChunks = options.include === 'allAssets'
      // Handle all chunks.
      ? extractedChunks
      // Only handle chunks imported by this HtmlWebpackPlugin.
      : extractedChunks.filter((chunk) => doesChunkBelongToHTML({
        chunk,
        compilation,
        htmlAssetsChunks: Object.values(htmlPluginData.assets.chunks)
      }))

    // Flatten the list of files.
    const allFiles = htmlChunks.reduce((accumulated, chunk) => {
      return accumulated.concat(chunk.files)
    }, [])
    const uniqueFiles = new Set(allFiles)
    const filteredFiles = [...uniqueFiles].filter(file => {
      return (
        !this.options.fileWhitelist ||
        this.options.fileWhitelist.some(regex => regex.test(file))
      )
    }).filter(file => {
      return (
        !this.options.fileBlacklist ||
        this.options.fileBlacklist.every(regex => !regex.test(file))
      )
    })
    // Sort to ensure the output is predictable.
    const sortedFilteredFiles = filteredFiles.sort()

    const links = []
    const publicPath = compilation.outputOptions.publicPath || ''
    for (const file of sortedFilteredFiles) {
      const href = `${publicPath}${file}`

      const attributes = {
        href,
        rel: options.rel
      }

      // If we're preloading this resource (as opposed to prefetching),
      // then we need to set the 'as' attribute correctly.
      if (options.rel === 'preload') {
        attributes.as = determineAsValue({
          href,
          file,
          optionsAs: options.as
        })

        // On the off chance that we have a cross-origin 'href' attribute,
        // set crossOrigin on the <link> to trigger CORS mode. Non-CORS
        // fonts can't be used.
        if (attributes.as === 'font') {
          attributes.crossorigin = ''
        }
      }

      links.push({
        tagName: 'link',
        attributes
      })
    }

    this.resourceHints = links
    return htmlPluginData
  }

  apply (compiler) {
    const skip = data => {
      const htmlFilename = data.plugin.options.filename
      const exclude = this.options.excludeHtmlNames
      const include = this.options.includeHtmlNames
      return (
        (include && !(include.includes(htmlFilename))) ||
        (exclude && exclude.includes(htmlFilename))
      )
    }

    compiler.hooks.compilation.tap(
      this.constructor.name,
      compilation => {
        compilation.hooks.htmlWebpackPluginBeforeHtmlProcessing.tap(
          this.constructor.name,
          (htmlPluginData) => {
            if (skip(htmlPluginData)) {
              return
            }
            this.generateLinks(compilation, htmlPluginData)
          }
        )

        compilation.hooks.htmlWebpackPluginAlterAssetTags.tap(
          this.constructor.name,
          (htmlPluginData) => {
            if (skip(htmlPluginData)) {
              return
            }
            if (this.resourceHints) {
              htmlPluginData.head = [
                ...this.resourceHints,
                ...htmlPluginData.head
              ]
            }
            return htmlPluginData
          }
        )
      }
    )
  }
}

module.exports = PreloadPlugin
