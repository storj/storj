# Vue Parser

![Important](https://travis-ci.org/prograhammer/vue-parser.svg?branch=master "Travis CI badge")

A simple way to parse a `.vue` file's contents (single file component) to get specific content from a
 tag like a template, script, or style tag. 
The returned content can be padded repeatedly with a specific string so that line numbers are retained
 (ie. good for reporting linting errors). You can also get the full [parse5](https://github.com/inikulin/parse5/blob/master/lib/index.d.ts#L193) 
 node/element which includes the location info of where the tag content is located. 

# Basic Usage

```javascript
import vueParser from 'vue-parser'

const vueFileContents = `
<template lang="pug">
.hello
  h1 {{ msg }}
</template>

<script lang="js">
export default {
  name: 'Hello',

  data () {
    return {
      msg: 'Hello World!'
    }
  }

}
</script>

<style>
h1 {
  font-weight: normal;
}
</style>
`

const myScriptContents = vueParser.parse(vueFileContents, 'script', { lang: ['js', 'jsx'] })

console.log(myScriptContents)

```

The console output would like like this 
(notice default padding string is `// ` since comments are typically ignored by linters/compilers/etc.):  

```text
// 
// 
// 
// 
// 
// 
export default {
  name: 'Hello',

  data () {
    return {
      msg: 'Hello World!'
    }
  }

}
```
