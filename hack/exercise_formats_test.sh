#!/bin/sh
# Holds hack/exercise_formats.sh's table step to canned `go test -json` output. It needs no corpus.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
t=$(mktemp -d); trap 'rm -rf "$t"' EXIT
fail() { echo "FAIL: $*" >&2; exit 1; }

# One board per verdict. royalblue passes its test, since the baseline holds its read failure, and
# its row must still say it does not read. Its error carries a quote, which go test -json escapes.
cat >"$t/kicad.json" <<'EOF'
{"Action":"run","Package":"p","Test":"TestOracleCorpus"}
{"Action":"run","Package":"p","Test":"TestOracleCorpus/cynthion"}
{"Action":"output","Package":"p","Test":"TestOracleCorpus/cynthion","Output":"    oracle_corpus_test.go:131: read: schematic 300 parts and 400 nets, board 300 parts and 400 nets\n"}
{"Action":"output","Package":"p","Test":"TestOracleCorpus/cynthion","Output":"    oracle_corpus_test.go:58: compared: which pins share a net\n"}
{"Action":"output","Package":"p","Test":"TestOracleCorpus/cynthion","Output":"    oracle_corpus_test.go:59: gaps: 0\n"}
{"Action":"pass","Package":"p","Test":"TestOracleCorpus/cynthion","Elapsed":1}
{"Action":"run","Package":"p","Test":"TestOracleCorpus/jetson"}
{"Action":"output","Package":"p","Test":"TestOracleCorpus/jetson","Output":"    oracle_corpus_test.go:131: read: schematic 9 parts and 8 nets, board 9 parts and 8 nets\n"}
{"Action":"output","Package":"p","Test":"TestOracleCorpus/jetson","Output":"    oracle_corpus_test.go:59: gaps: 10\n"}
{"Action":"pass","Package":"p","Test":"TestOracleCorpus/jetson","Elapsed":1}
{"Action":"run","Package":"p","Test":"TestOracleCorpus/royalblue54L-feather"}
{"Action":"output","Package":"p","Test":"TestOracleCorpus/royalblue54L-feather","Output":"    oracle_corpus_test.go:128: read: schematic 12 parts and 30 nets; does not read, the board: sexpr: \"(\"\n"}
{"Action":"output","Package":"p","Test":"TestOracleCorpus/royalblue54L-feather","Output":"    oracle_corpus_test.go:59: gaps: 1\n"}
{"Action":"pass","Package":"p","Test":"TestOracleCorpus/royalblue54L-feather","Elapsed":1}
{"Action":"run","Package":"p","Test":"TestOracleCorpus/hackrf-one"}
{"Action":"output","Package":"p","Test":"TestOracleCorpus/hackrf-one","Output":"    oracle_corpus_test.go:59: gaps: 3\n"}
{"Action":"fail","Package":"p","Test":"TestOracleCorpus/hackrf-one","Elapsed":1}
{"Action":"fail","Package":"p","Test":"TestOracleCorpus","Elapsed":4}
{"Action":"fail","Package":"p","Elapsed":4}
EOF
# A test with no subtests names its file in a file: line.
cat >"$t/pour.json" <<'EOF'
{"Action":"run","Package":"q","Test":"TestPour"}
{"Action":"output","Package":"q","Test":"TestPour","Output":"    pour_test.go:70: file: board.odb.tgz\n"}
{"Action":"output","Package":"q","Test":"TestPour","Output":"    pour_test.go:71: read: the job\n"}
{"Action":"output","Package":"q","Test":"TestPour","Output":"    pour_test.go:72: gaps: 0\n"}
{"Action":"pass","Package":"q","Test":"TestPour","Elapsed":1}
EOF
# A package that does not build reports no test at all.
cat >"$t/broken.json" <<'EOF'
{"Action":"output","Package":"r","Output":"FAIL\tr [build failed]\n"}
{"Action":"fail","Package":"r","Elapsed":0}
EOF

[ -x "$here/exercise_formats.sh" ] || fail "hack/exercise_formats.sh is missing or not executable"
"$here/exercise_formats.sh" rows KiCad "$t/kicad.json" >"$t/kicad.tsv"
"$here/exercise_formats.sh" rows "ODB++ DRC" "$t/pour.json" >"$t/pour.tsv"
"$here/exercise_formats.sh" rows Broken "$t/broken.json" >"$t/broken.tsv"

tab=$(printf '\t')
want() { grep -qxF "$2" "$t/$1" || { echo "got $1:" >&2; cat "$t/$1" >&2; fail "no row: $2"; }; }
want kicad.tsv "KiCad${tab}cynthion${tab}schematic 300 parts and 400 nets, board 300 parts and 400 nets${tab}which pins share a net${tab}pass"
want kicad.tsv "KiCad${tab}jetson${tab}schematic 9 parts and 8 nets, board 9 parts and 8 nets${tab}-${tab}known gaps (10)"
want kicad.tsv "KiCad${tab}royalblue54L-feather${tab}schematic 12 parts and 30 nets; does not read, the board: sexpr: \"(\"${tab}-${tab}does not read"
want kicad.tsv "KiCad${tab}hackrf-one${tab}-${tab}-${tab}FAIL"
want pour.tsv "ODB++ DRC${tab}board.odb.tgz${tab}the job${tab}-${tab}pass"
want broken.tsv "Broken${tab}-${tab}-${tab}-${tab}FAIL (r did not run)"
# The parent test is no row of its own once it has subtests.
[ "$(wc -l <"$t/kicad.tsv" | tr -d ' ')" = 4 ] || { cat "$t/kicad.tsv" >&2; fail "want 4 KiCad rows"; }
[ "$(wc -l <"$t/pour.tsv" | tr -d ' ')" = 1 ] || fail "want 1 pour row"

cat "$t/kicad.tsv" "$t/pour.tsv" | "$here/exercise_formats.sh" table >"$t/table"
head -1 "$t/table" | grep -q '^format  *corpus file  *read  *compared  *verdict$' || { cat "$t/table" >&2; fail "table header"; }
grep -q '^KiCad  *royalblue54L-feather  .*  does not read$' "$t/table" || { cat "$t/table" >&2; fail "table row for royalblue"; }
[ "$(wc -l <"$t/table" | tr -d ' ')" = 7 ] || { cat "$t/table" >&2; fail "want a header, a rule and 5 rows"; }
echo "ok   exercise_formats turns go test -json into one row per corpus file, and keeps a read failure visible"
