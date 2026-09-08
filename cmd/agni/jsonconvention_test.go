package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// jsonEncoderExceptions are the files allowed to hand-roll json, each for a reason stated where the
// type is defined. C31's whole value is that this list is short and argued.
var jsonEncoderExceptions = map[string]string{
	"intake.go": "intake.Skeleton's confidentiality guarantee is structural (C16); see the type's doc comment",
}

// C31's Verify. A command that hand-rolls its json is emitting a second shape of an answer the wire
// already has a message for, which is C26's silent-drift failure one layer out.
//
// It reads the SOURCE rather than running the commands, deliberately. Running them would prove the
// shape of the answers a fixture happens to produce; the claim is about which encoder a code path
// reaches for, and that is a property of the code.
//
// WHAT IT CANNOT SEE, stated because it caught me: an encoder reached through a HELPER in another
// package. `query` used to call `report.TableJSON`, and putting that call back does not fail this
// test, because the `json.NewEncoder` was one package away. The answer was to delete `TableJSON`
// rather than to widen the pattern, since it had no caller left and a function that exists only to be
// grepped for is worse than one that does not exist. A future helper of the same shape would slip
// through the same way, so the real defence is that this test's exception list is short enough to
// read and the next person notices a new one.
func TestEveryJSONFormatEmitsAProto(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	// A json encoder constructed anywhere in a command file. encoding/json is also used for READING
	// (importresults decodes a stored document), which is fine and not what this looks for.
	encoder := regexp.MustCompile(`json\.NewEncoder|json\.Marshal(Indent)?\(`)
	var offenders []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		if _, ok := jsonEncoderExceptions[filepath.Base(f)]; ok {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if encoder.Match(src) {
			offenders = append(offenders, filepath.Base(f))
		}
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Errorf("these hand-roll json: %v\n\n"+
			"C31: a --format json emits protojson of the message that command's rpc returns, so a "+
			"script reading the CLI and a client reading the API parse one shape. If this file's "+
			"answer has no wire message yet, adding one is the work. If it genuinely must not have "+
			"one, add it to jsonEncoderExceptions with the reason, and put the reasoning on the TYPE "+
			"where someone adding a field will read it.", offenders)
	}
}

// The exception list is only worth having if it stays small and each entry is real, so a stale entry
// naming a file that no longer hand-rolls anything has to fail rather than quietly widen the rule.
func TestJSONExceptionsAreStillNeeded(t *testing.T) {
	encoder := regexp.MustCompile(`json\.NewEncoder|json\.Marshal(Indent)?\(`)
	for name, why := range jsonEncoderExceptions {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Errorf("exception %q names a file that is not here: %v", name, err)
			continue
		}
		if !encoder.Match(src) {
			t.Errorf("%s no longer hand-rolls json, so its exception (%q) is stale and should go", name, why)
		}
	}
}
