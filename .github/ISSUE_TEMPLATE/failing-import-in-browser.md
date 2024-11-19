---
name: A failing module import in Browser
about: Submit a report if a module fails to import in Browser.
title: 'Failed to import -'
labels: browser
---

## Failing module

- **GitHub**: https://github.com/my/repo
- **npm**: https://npmjs.com/package/my_package

```js
import { something } from "https://esm.sh/my_module"
```

## Error message

After `onload` I got this:

```
/* your error log here */
```

## Additional info

- **Browser info**:
