# comment-no-empty

Disallow empty comments.

<!-- prettier-ignore -->
```scss
    /* */
    //
```

## Options

### `true`

The following patterns are considered violations:

<!-- prettier-ignore -->
```scss
/**/
```

<!-- prettier-ignore -->
```scss
/* */
```

<!-- prettier-ignore -->
```scss
/*

 */
```

<!-- prettier-ignore -->
```scss
//
```

<!-- prettier-ignore -->
```scss
width: 10px; //
```

The following patterns are _not_ considered violations:

<!-- prettier-ignore -->
```scss
/* comment */
```

<!-- prettier-ignore -->
```scss
/*
 * Multi-line Comment
**/
```