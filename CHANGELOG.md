#### `0.11.0`:

* Added raw input type for any context var: `var4 = {path = [[], ""], type = "raw"}`
   - `wholeRaw` stores the entirety of a read file as a string value for the given context key
   -  any subpaths provided when `var.type = "raw"` will return an error
*  Renamed `--out=raw` to `--out=list` to better clarify the output type and distinguish it from the read type of  `raw`

#### `0.10.0`:
CLI signature has changed from `cogs gen <ctx> <cog-file> [options]` to `cogs gen <cog-file> <ctx>... [options]`
This will allow multiple contexts to be pulled from the same cog file.

This is a breaking change.

* HTTP headers now use canonical keys
* dotenv output now looks at http headers to decide the value of the output string
