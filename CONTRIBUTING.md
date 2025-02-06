# Contributing to esm.sh

Welcome, and thank you for taking time in contributing to esm.sh project!

## Development Setup

You will need [Golang](https://golang.org/)(1.22+) and [Deno](https://deno.land)(1.45+) installed on a macOS or Linux-based machine.

1. Fork this repository to your own GitHub account.
2. Clone the repository to your local device.
3. Create a new branch (`git checkout -b BRANCH_NAME`).
4. Change code then run tests
5. Push your branch to GitHub after **all tests passed**.
6. Make a [pull request](https://github.com/esm-dev/esm.sh/pulls).
7. Merge to master branch by our maintainers.

## Configration

Create a `config.json` file in the project root directory following the example below:

```jsonc
// config.json
{
  "port": 8080,
  "workDir": ".esmd",
  "npmRegistry": "https://registry.npmjs.org/", // change to your own registry if needed
  "npmToken": "xxxxxx", // remove this line if you don't have one
}
```

More server options please check [config.exmaple.jsonc](./config.example.jsonc).

## Running the Server from Source Code

```bash
make dev/server
```

Then you can import `React` from "http://localhost:8080/react"

## Running Server Integration Tests

We use [Deno](https://deno.land) to run all the integration testing cases. Make sure you have Deno installed on your computer.

```bash
# Run all tests
make test/server

# Run a specific test
make test/server dir=react-18
```

To add a new integration test case, copy the [test/.template](./test/.template) directory and rename it to your case name.

```bash
# copy the testing template
cp -r test/.template test/fix-xxx
# edit the test code
nvim test/fix-xxx/test.ts
# run the test
make test/server dir=fix-xxx
```
