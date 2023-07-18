# A-go-y!

A-go-y! (pronounced Ahoy!), is an extremely hacky CLI created to get the filesizes from a list
of magnets written on a single txt (if you know, well, you know ;))

Useful to make a decent-enough estimate on the quality of certain videos you can download.

To install simply run `go install github.com/etra0/a-go-y`.

## Usage

```
-keywords string
        List of keywords to search for in the torrent files
  -magnets string
        File that contains the list of magnets to search
  -timeout int
        Timeout in seconds to wait for a response (default 120)
  -verbose
        Verbose output
```