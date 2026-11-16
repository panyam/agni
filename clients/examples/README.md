# Client examples

One small program per language, each calling a shared `agni serve` through that language's client
(mission #1625). Each checks the server, ingests a design, asks it three questions, and prints one
line of JSON summing up the answers. `make exercise-clients` runs every `<language>/run` here
against one server and compares each line with the CLI's answers.

| Language | Example | Ticket |
| --- | --- | --- |
| Go | [go/](go/) | the reference, #1628 |
| TypeScript | to come | #1630 |
| Rust | [rust/](rust/), through [clients/rust](../rust/) | #1631 |
| Java | to come | #1632 |
| C++ | to come | #1638 |

A `run` script takes `-server <url>` and `-design <mount:// URI>`, and its last line of output is the
summary. [Writing an agni client](../../docsite/content/build/client-contract.md) says what each
example and client does.
