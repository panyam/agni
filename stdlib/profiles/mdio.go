package profiles

import _ "embed"

//go:embed builtins/mdio.yaml
var mdioYAML []byte

// MDIO is the Ethernet PHY management bus: MDIO (bidirectional, open-drain, needs a pull-up) and MDC
// (the STA-sourced clock, which does not). MDIO is the anchor.
//
// It exists because the engine had exactly one pull-up rule and it answered one bus family by name.
// `i2cNamePattern` matches SDA/SCL at a token boundary and nothing else in the tree looked at a
// management-interface name, so a board whose PHY bus had no pull-up on either line reported nothing,
// from a rule whose entire subject is that failure (agni issue 516). MDIO is the same electrical
// question as I2C rather than an adjacent one: both lines idle high only because a resistor returns
// them there, and both sit undriven between transactions.
//
// The profile route rather than widening the built-in rule's pattern: doing both would double-report
// the same net from two rules, and this way the coverage is DATA. The cost is that a profile
// requirement's finding names the net while the built-in now carries PullUpPathToRail's hops, so a
// reviewer asking "show me the pull-up" gets a witness on I2C and a bare net name here. That
// asymmetry is the remaining half of issue 516 and is not addressed by this profile.
var MDIO = mustParse(mdioYAML)
