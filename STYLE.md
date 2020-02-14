# Gitlab-Runner Code Style

## Areas Where We Can Improve

These are areas where we know we're falling short of the typical Go guidelines and even our own currently expected and enforced style. If you're working on something, feel free to update the parts of the code you're working on to match the preferred guidelines.

Otherwise, opening an MR to improve just the style is preferred to help keep MRs having a single context.

* The [Go guidelines](https://github.com/golang/go/wiki/Errors) state that errors should typically start with a `lower case`, many of our errors start with an `uppercase`
