## house.pmic_rail

### What it is

`house.pmic_rail(net)` holds for each rail this team's PMIC drives. The team names every PMIC
output `PMIC_<rail>`, so the member is a rail (`net.rail`) whose name starts with `PMIC_`. Control
nets such as `PMIC_EN` and `PMIC_PG` share the prefix and are not rails, so they are left out.

### For hardware engineers

The PMIC rails are the first thing powered at bring-up and the first thing measured. Asking for the
ones without a test point, `house.pmic_rail(?n), not house.pmic_probe_point(?n)`, finds the rail
someone will end up probing at a capacitor pad.

### For software engineers

A filter over `net.rail` by name, defined in this project's `lib/house.dl`. It is a project member
rather than a shipped one because the naming convention is this team's.

### Datalog

```
house.pmic_rail(?n), not house.pmic_probe_point(?n) => ?n
```
