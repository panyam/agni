#!/bin/sh
# The format-fidelity mission's exercise (agni issues 1710 and 1714). It runs every format oracle,
# keeps going past a failure, and prints one row per format and corpus file.
#
#   hack/exercise_formats.sh                     run every oracle and print the table
#   hack/exercise_formats.sh rows FORMAT FILE    turn one `go test -json` log into table rows
#   hack/exercise_formats.sh table               align rows read from stdin
#
# A row is five tab-separated fields: format, corpus file, what read, what was compared, verdict.
# An oracle test logs `read:`, `compared:`, `gaps:` and optionally `file:` lines for each corpus
# file, one subtest per file. The verdict is FAIL when the test failed. A passing test still gets
# "does not read" when its read line says so, since a baseline can hold a read failure. A count of
# gaps the baseline holds gives "known gaps (N)". Otherwise the verdict is pass.
#
# BROWSER=1 adds the browser halves of exercise-allegro and exercise-altium. GO names the go binary.
# The script exits 1 when any row fails or does not read.
set -u

rows() {
  awk -v fmt="$1" '
    # jstr returns the JSON string value of key in line, unescaped. go test -json writes one object
    # per line and escapes only what JSON requires, plus <, > and & as <, > and &.
    function jstr(line, key,    i, c, out, n, h) {
      i = index(line, "\"" key "\":\"")
      if (i == 0) return ""
      i += length(key) + 4
      out = ""
      n = length(line)
      while (i <= n) {
        c = substr(line, i, 1)
        if (c == "\"") break
        if (c == "\\") {
          i++
          c = substr(line, i, 1)
          if (c == "n") c = "\n"
          else if (c == "t") c = " "
          else if (c == "u") {
            h = substr(line, i + 1, 4); i += 4
            c = (h == "003c") ? "<" : (h == "003e") ? ">" : (h == "0026") ? "&" : "?"
          }
        }
        out = out c
        i++
      }
      return out
    }
    function field(name, def) { return (name in val) ? val[name] : def }
    {
      act = jstr($0, "Action"); t = jstr($0, "Test"); pkg = jstr($0, "Package")
      if (t == "") {
        if (act == "fail") pkgfail[pkg] = 1
        if (pkg != "" && !(pkg in pkgseen)) { pkgseen[pkg] = 1; pkgs[++np] = pkg }
        next
      }
      if (!(t in seen)) { seen[t] = 1; order[++nt] = t }
      if (index(t, "/")) { parent[substr(t, 1, index(t, "/") - 1)] = 1 }
      testpkg[t] = pkg
      if (act == "pass" || act == "fail" || act == "skip") result[t] = act
      if (act == "output") {
        o = jstr($0, "Output")
        sub(/\n$/, "", o)
        sub(/^ +/, "", o)
        sub(/^[^ :]+\.go:[0-9]+: /, "", o)
        if (match(o, /^(file|read|compared|gaps): /)) {
          k = substr(o, 1, RLENGTH - 2)
          val[t SUBSEP k] = substr(o, RLENGTH + 1)
          gsub(/\t/, " ", val[t SUBSEP k])
        }
      }
    }
    END {
      for (j = 1; j <= nt; j++) {
        t = order[j]
        if (t in parent) continue
        tested[testpkg[t]] = 1
        name = t; sub(/^[^\/]*\//, "", name)
        f = ((t SUBSEP "file") in val) ? val[t SUBSEP "file"] : name
        r = ((t SUBSEP "read") in val) ? val[t SUBSEP "read"] : "-"
        c = ((t SUBSEP "compared") in val) ? val[t SUBSEP "compared"] : "-"
        g = ((t SUBSEP "gaps") in val) ? val[t SUBSEP "gaps"] + 0 : 0
        res = (t in result) ? result[t] : "fail"
        if (res == "fail") v = "FAIL"
        else if (res == "skip") v = "skipped"
        else if (index(r, "does not read")) v = "does not read"
        else if (g > 0) v = "known gaps (" g ")"
        else v = "pass"
        printf "%s\t%s\t%s\t%s\t%s\n", fmt, f, r, c, v
      }
      # A package that did not build, or ran no test, still gets a row.
      for (j = 1; j <= np; j++) {
        if (!(pkgs[j] in tested)) {
          p = pkgs[j]; sub(/.*\//, "", p)
          printf "%s\t-\t-\t-\t%s\n", fmt, (pkgs[j] in pkgfail) ? "FAIL (" p " did not run)" : "FAIL (" p " ran no test)"
        }
      }
    }
  ' "$2"
}

table() {
  awk -F'\t' '
    { n++; for (i = 1; i <= 5; i++) { cell[n, i] = $i; if (length($i) > w[i]) w[i] = length($i) } }
    END {
      split("format|corpus file|read|compared|verdict", h, "|")
      for (i = 1; i <= 5; i++) if (length(h[i]) > w[i]) w[i] = length(h[i])
      line(h)
      for (i = 1; i <= 5; i++) { r[i] = ""; while (length(r[i]) < w[i]) r[i] = r[i] "-" }
      line(r)
      for (k = 1; k <= n; k++) { for (i = 1; i <= 5; i++) c[i] = cell[k, i]; line(c) }
    }
    function line(a,    i, s) {
      s = ""
      for (i = 1; i <= 4; i++) s = s sprintf("%-" w[i] "s  ", a[i])
      print s a[5]
    }
  '
}

case "${1:-}" in
  rows) rows "$2" "$3"; exit ;;
  table) table; exit ;;
  "") ;;
  *) echo "usage: $0 [rows FORMAT FILE | table]" >&2; exit 2 ;;
esac

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
GO=${GO:-go}
out=$(mktemp -d); trap 'rm -rf "$out"' EXIT
tsv="$out/rows.tsv"
: >"$tsv"

step() { echo "==> $*" >&2; }

# An oracle test, as rows. Its own output goes to the json file and nowhere else.
oracle() { # format env-var package test
  step "$4 ($3)"
  env "$2=1" "$GO" test "$3" -run "^$4\$" -count=1 -json >"$out/run.json" 2>"$out/run.err"
  if [ ! -s "$out/run.json" ]; then
    printf '%s\t-\t-\t-\tFAIL (%s did not run)\n' "$1" "$4" >>"$tsv"
    cat "$out/run.err" >&2
    return
  fi
  rows "$1" "$out/run.json" >>"$tsv"
}

# An example's Go half, as one row. An example compares nothing with an oracle, so its row says
# what it read and what it found.
example() { # format dir read-pattern compared
  step "examples/$2"
  (cd "examples/$2" && AGNI_EXAMPLE_DESIGN= "$GO" run . --non-interactive) >"$out/ex.out" 2>&1
  code=$?
  file=$(sed -n 's/^Selected \(.*\)\.$/\1/p; s/^Layout: //p' "$out/ex.out" | tail -1)
  file=${file##*/}
  parts=$(awk '$1 == "components:" { print $2 }' "$out/ex.out" | tail -1)
  nets=$(awk '$1 == "nets:" { print $2 }' "$out/ex.out" | tail -1)
  read="${parts:-?} parts and ${nets:-?} nets"
  compared=$4
  if [ -n "$3" ]; then
    compared="$compared: $(grep -E "$3" "$out/ex.out" | tr -s ' ' | paste -sd ';' - | sed 's/;/; /g')"
  fi
  if [ "$code" = 0 ]; then v="ran"; else v="FAIL (exit $code)"; tail -20 "$out/ex.out" >&2; fi
  printf '%s\t%s\t%s\t%s\t%s\n' "$1" "${file:--}" "$read" "$compared" "$v" >>"$tsv"
}

# A browser half of an exercise, as one row.
browser() { # format test-filter file
  step "web/browser/native-board.spec.ts -t $2"
  if (cd web && pnpm exec vitest run -c vitest.browser.config.ts browser/native-board.spec.ts -t "$2") >"$out/br.out" 2>&1; then
    v=pass
  else
    v=FAIL; tail -20 "$out/br.out" >&2
  fi
  printf '%s (browser)\t%s\topened in the viewer\ta pad click selects its part\t%s\n' "$1" "$3" "$v" >>"$tsv"
}

oracle KiCad AGNI_ORACLE ./readers/kicad/ TestOracleCorpus
step "fetch odb-kicad"
./hack/fetch_samples.sh odb-kicad >&2 || printf 'ODB++\t-\t-\t-\tFAIL (odb-kicad did not fetch)\n' >>"$tsv"
oracle ODB++ AGNI_ODB_ORACLE ./readers/formats/ TestODBOracleCorpus
oracle "ODB++ DRC" AGNI_ODB_ORACLE ./stdlib/rules/builtin/ TestPourClearanceFindsKiCadsZonePairs
example Allegro allegro-board "" "no oracle; the example reads the board and runs the checks"
example Altium altium-board "" "no oracle; the example reads the board and runs the checks"
example "ODB++ example" odb-verification "^(Components|Nets):" "diff against its schematic netlist"
if [ "${BROWSER:-0}" = 1 ]; then
  step "make ui"
  make ui >&2 || true
  browser Allegro Allegro BeagleBone-AI.brd
  browser Altium Altium master.PcbDoc
fi
for f in xschem gEDA OrCAD EDIF IPC-2581; do
  printf '%s\t-\t-\tno oracle\tno oracle\n' "$f" >>"$tsv"
done

echo
table <"$tsv"
bad=$(awk -F'\t' '$5 ~ /^FAIL/ || $5 == "does not read"' "$tsv" | wc -l | tr -d ' ')
echo
if [ "$bad" != 0 ]; then
  echo "$bad rows fail or do not read."
  exit 1
fi
echo "Every row with an oracle passes."
