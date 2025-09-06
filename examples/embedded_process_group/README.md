# Embedded process group example

This example demonstrates how to manage a group of processes together using provisr's public wrapper API.

It starts two members:

- web: a short-lived shell command
- worker: another command with Instances=2, showing multiple instances in a single group member

Run it with:

```shell
go run ./examples/embedded_process_group
```

Output:

- Prints JSON status map for each member (including instance PIDs) after starting
- Stops the group and prints the statuses again
