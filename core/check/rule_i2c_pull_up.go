package check

import (
	"regexp"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// i2cPullUp flags an I2C net (SDA/SCL) with no pull-up resistor. See Detail.
var i2cPullUp = &Rule{
	Name:       "i2c-pull-up",
	Severity:   "error",
	Summary:    "An I2C net (SDA/SCL) has no pull-up resistor.",
	Impact:     "I2C pins are open-drain: they can only pull the line low. With no pull-up the line never returns high, so the bus is stuck and nothing on it communicates. It is a total-function failure and a recurring field bug.",
	Primitives: []string{"select", "pattern", "traverse", "exists"},
	Reads:      []string{"net.names", "on_net", "component.class"},
	Tags: map[string]string{
		KeyCategory:     CategoryConnectivity,
		KeyTier:         "R",
		KeyDistribution: DistPublicReference,
	},
	Detail: ruleDoc("i2c-pull-up"),
	Eval: func(m Model) []Finding {
		bad := Select(m.Nets(), func(n *ir.Net) bool {
			return isI2C(n.Name) && !Exists(n.Connections, func(c *ir.Connection) bool {
				return m.HasClass(c.ComponentRef, ClassResistor)
			})
		})
		return Report(bad, NetFinding("I2C net has no pull-up resistor"))
	},
}

// i2cNamePattern matches an SDA/SCL bus name at a TOKEN boundary, not as a substring: SDA/SCL must be
// bounded by a non-letter (start/end, separator, or a channel digit) on each side. RE2 has no
// lookaround, so the boundary is explicit character classes. This is what keeps SPI_SCLK (SPI clock)
// from reading as I2C — the trailing K is a letter, so SCL is not at a boundary (WS3-037). Case is
// folded before matching, so [^A-Z] is any non-letter.
var i2cNamePattern = regexp.MustCompile(`(^|[^A-Z])(SDA|SCL)([^A-Z]|$)`)

// isI2C matches the SDA/SCL naming convention at a token boundary. Matches SDA, SCL, I2C_SCL, SCL0,
// SDA_1; NOT SPI_SCLK, SCLK, MCLK, or MISCL (SDA/SCL abutting another letter).
func isI2C(name string) bool {
	return i2cNamePattern.MatchString(strings.ToUpper(name))
}

// i2cPullUpSpec is the rule's declarative twin (WS3-003).
var i2cPullUpSpec = &Spec{
	Over: "nets",
	Where: And{Xs: []Expr{
		Match{T: Fact{"net.names"}, Pattern: "(?i)(^|[^A-Z])(SDA|SCL)([^A-Z]|$)"},
		Not{X: ExistsIn{Over: "net.connections", Where: Cmp{L: Fact{"component.class"}, Op: "==", R: Lit{"resistor"}}}},
	}},
	Message: "I2C net has no pull-up resistor",
}
