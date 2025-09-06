# Embedded example

This example shows how to embed provisr in your Go program without importing any internal packages.

Run it with:

```shell
go run ./examples/embedded
```

What it does:

- Creates a provisr.Manager
- Sets a global environment variable
- Starts a short-lived process
- Prints status before and after stopping
