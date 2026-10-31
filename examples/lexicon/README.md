# lexicon

See the naming vocabulary a design is read with, why one net has its roles, and what a different
convention would change. It is the walkthrough form of `agni lexicon`.

## What it shows

- `LexiconService.GetLexicon`, every pattern of the effective lexicon with where it came from: built
  in, or the convention the server, the project, the design or the request supplied.
- `LexiconService.ExplainNet`, one net's roles with their reasons and the patterns its name matches.
- `LexiconService.CompareLexicons`, the nets whose roles change and the findings that appear or
  disappear when a candidate convention replaces the design's.
- The services composed the way the browser engine composes them, over `fshost`, so the example
  reads the design through the same ports a server does.

## Run it

```bash
make run          # plain text, prompts for the design and the net
make runquiet     # non-interactive, the defaults throughout
```

The default design is `../common/designs/lexicon-demo`, a sensor on its own regulator whose output is
named `SENSOR_3V3`. Its `design.yaml` carries the one rail pattern that makes that name a rail.
