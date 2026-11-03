package profiles

import _ "embed"

//go:embed builtins/mdio.yaml
var mdioYAML []byte

// MDIO is the Ethernet PHY management bus profile. MDIO is bidirectional and open-drain, so it needs
// a pull-up, and MDC is the STA-sourced clock, which does not. MDIO is the anchor.
//
// It is a profile rather than a wider built-in I2C pull-up pattern, since doing both would report the
// same net from two rules (agni issue 516). Its missing-pullup requirement compiles through
// pullupRule, so a verdict here names the same pull-up hops as the built-in's. See
// docsite/content/guide/interface-profiles.md#why-mdio-is-a-profile-and-not-a-wider-built-in.
var MDIO = mustParse(mdioYAML)
