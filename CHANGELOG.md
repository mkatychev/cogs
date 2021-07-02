#### `0.10.0`:
CLI signature has changed from `cogs gen <ctx> <cog-file> [options]` to `cogs gen <cog-file> <ctx>... [options]`
This will allow multiple contexts to be pulled from the same cog file.

This is a breaking change.

* HTTP headers now use canonical keys
* dotenv output now looks at http headers to decide the value of the output string
