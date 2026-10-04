# Revision audit

- base: `mount://audit/designs/gateway/gateway.edn`
- head: `mount://audit/designs/gateway/gateway-rev-b.edn`

## Diff

17 rows

| change_class | subject | old_name | field | old_value | new_value | added | removed | old_source_file | new_source_file | match_old_coverage | match_old_coverage_significant | match_new_coverage_significant |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| component-added | R4 |  |  |  |  |  |  |  |  |  |  |  |
| component-added | R5 |  |  |  |  |  |  |  |  |  |  |  |
| net-hard | I2C_SCL |  |  |  |  | R4.2 |  | designs/gateway/gateway.edn | designs/gateway/gateway-rev-b.edn |  |  |  |
| net-hard | I2C_SDA |  |  |  |  | R5.2 |  | designs/gateway/gateway.edn | designs/gateway/gateway-rev-b.edn |  |  |  |
| net-hard | PMIC_CORE_3V3 |  |  |  |  | R4.1\|R5.1 |  | designs/gateway/gateway.edn | designs/gateway/gateway-rev-b.edn |  |  |  |

## Pin to net

60 rows

| part | pin | net | provenance |
|---|---|---|---|
| C1 | 1 | PMIC_CORE_3V3 | designs/gateway/gateway-rev-b.edn |
| C1 | 2 | GND | designs/gateway/gateway-rev-b.edn |
| C2 | 1 | PMIC_CORE_3V3 | designs/gateway/gateway-rev-b.edn |
| C2 | 2 | GND | designs/gateway/gateway-rev-b.edn |
| C3 | 1 | PMIC_IO_1V8 | designs/gateway/gateway-rev-b.edn |

## Part numbers

19 rows

| part | mpn | provenance |
|---|---|---|
| C1 | ACME-CAP-22U | designs/gateway/gateway-rev-b.edn |
| C2 | ACME-CAP-100N | designs/gateway/gateway-rev-b.edn |
| C3 | ACME-CAP-10U | designs/gateway/gateway-rev-b.edn |
| C4 | ACME-CAP-100N | designs/gateway/gateway-rev-b.edn |
| C5 | ACME-CAP-18P | designs/gateway/gateway-rev-b.edn |

## Test points per net

15 rows

| net | tps | provenance |
|---|---|---|
| CAN1_CANH | 0 | designs/gateway/gateway-rev-b.edn:CAN1_CANH |
| CAN1_CANL | 0 | designs/gateway/gateway-rev-b.edn:CAN1_CANL |
| CAN1_RXD | 0 | designs/gateway/gateway-rev-b.edn:CAN1_RXD |
| CAN1_TXD | 0 | designs/gateway/gateway-rev-b.edn:CAN1_TXD |
| CLK_IN | 0 | designs/gateway/gateway-rev-b.edn:CLK_IN |

## One test point away

6 rows

| mpn | part | probed | unprobed | provenance |
|---|---|---|---|---|
| ACME-CAP-22U | C1 | PMIC_CORE_3V3 | GND | designs/gateway/gateway-rev-b.edn ; designs/gateway/gateway-rev-b.edn:GND ; designs/gateway/gateway-rev-b.edn:PMIC_CORE_3V3 |
| ACME-CAP-100N | C2 | PMIC_CORE_3V3 | GND | designs/gateway/gateway-rev-b.edn ; designs/gateway/gateway-rev-b.edn:GND ; designs/gateway/gateway-rev-b.edn:PMIC_CORE_3V3 |
| ACME-CAP-10U | C3 | PMIC_IO_1V8 | GND | designs/gateway/gateway-rev-b.edn ; designs/gateway/gateway-rev-b.edn:GND ; designs/gateway/gateway-rev-b.edn:PMIC_IO_1V8 |
| ACME-CAP-100N | C4 | PMIC_IO_1V8 | GND | designs/gateway/gateway-rev-b.edn ; designs/gateway/gateway-rev-b.edn:GND ; designs/gateway/gateway-rev-b.edn:PMIC_IO_1V8 |
| ACME-RES-4K7 | R4 | PMIC_CORE_3V3 | I2C_SCL | designs/gateway/gateway-rev-b.edn ; designs/gateway/gateway-rev-b.edn:I2C_SCL ; designs/gateway/gateway-rev-b.edn:PMIC_CORE_3V3 |

## Part numbers never probed

9 rows

| mpn | list(distinct part) | provenance |
|---|---|---|
| ACME-CAP-100N | C2 C4 | designs/gateway/gateway-rev-b.edn ; designs/gateway/gateway-rev-b.edn:GND ; designs/gateway/gateway-rev-b.edn:PMIC_CORE_3V3 ; designs/gateway/gateway-rev-b.edn:PMIC_IO_1V8 |
| ACME-CAP-10U | C3 | designs/gateway/gateway-rev-b.edn ; designs/gateway/gateway-rev-b.edn:GND ; designs/gateway/gateway-rev-b.edn:PMIC_IO_1V8 |
| ACME-CAP-18P | C5 C6 | designs/gateway/gateway-rev-b.edn ; designs/gateway/gateway-rev-b.edn:CLK_IN ; designs/gateway/gateway-rev-b.edn:CLK_OUT ; designs/gateway/gateway-rev-b.edn:GND |
| ACME-CAP-22U | C1 | designs/gateway/gateway-rev-b.edn ; designs/gateway/gateway-rev-b.edn:GND ; designs/gateway/gateway-rev-b.edn:PMIC_CORE_3V3 |
| ACME-RES-10K | R2 R3 | designs/gateway/gateway-rev-b.edn ; designs/gateway/gateway-rev-b.edn:MCU_NRST ; designs/gateway/gateway-rev-b.edn:PMIC_EN ; designs/gateway/gateway-rev-b.edn:PMIC_MAIN_12V0 ; designs/gateway/gateway-rev-b.edn:PMIC_PG |

## PMIC rails

3 rows

| net | tps | provenance |
|---|---|---|
| PMIC_CORE_3V3 | 1 | designs/gateway/gateway-rev-b.edn ; designs/gateway/gateway-rev-b.edn:PMIC_CORE_3V3 |
| PMIC_IO_1V8 | 1 | designs/gateway/gateway-rev-b.edn ; designs/gateway/gateway-rev-b.edn:PMIC_IO_1V8 |
| PMIC_MAIN_12V0 | 0 | designs/gateway/gateway-rev-b.edn:PMIC_MAIN_12V0 |

## Review

15 rows

| area | id | title | outcome | note | findings | unmet |
|---|---|---|---|---|---|---|
| Power | P1 | every rail carries a bulk capacitor | pass |  |  |  |
| Power | P2 | every rail carries decoupling | fail |  | decoupling-present=net:PMIC_MAIN_12V0 |  |
| Power | P3 | the input rail is protected against reverse polarity | fail |  | reverse-blocking-absent=net:PMIC_MAIN_12V0 |  |
| Power | P4 | no part is operated above its absolute-maximum supply voltage | provisional |  | supply-exceeds-abs-max=component:U2 |  |
| Power | P5 | every rail is probeable during bring-up | fail |  | test-point-coverage=net:GND\|test-point-coverage=net:PMIC_MAIN_12V0 |  |

## Review summary

1 rows

| total | covered | answered | pass | fail | provisional |
|---|---|---|---|---|---|
| 15 | 13 | 13 | 5 | 7 | 1 |

## Findings

24 rows

| severity | inconclusive | rule | kind | subject | pin | net_id | message | source_file | native_id | context |
|---|---|---|---|---|---|---|---|---|---|---|
| error | false | copper-clearance | net | CAN1_CANH |  |  | copper of "CAN1_CANH" and "I2C_SCL" closer than 0.127mm at 1 place(s); worst gap -0.175mm near (5.00, -84.00)mm |  |  | neighbour=I2C_SCL |
| error | false | copper-clearance | net | CAN1_CANL |  |  | copper of "CAN1_CANL" and "I2C_SDA" closer than 0.127mm at 1 place(s); worst gap -0.250mm near (5.00, -85.50)mm |  |  | neighbour=I2C_SDA |
| error | false | copper-clearance | net | CAN1_RXD |  |  | copper of "CAN1_RXD" and "PMIC_CORE_3V3" closer than 0.127mm at 1 place(s); worst gap -0.250mm near (5.00, -79.50)mm |  |  | neighbour=PMIC_CORE_3V3 |
| error | false | copper-clearance | net | CAN1_RXD |  |  | copper of "CAN1_RXD" and "PMIC_PG" closer than 0.127mm at 1 place(s); worst gap -0.250mm near (5.00, -79.50)mm |  |  | neighbour=PMIC_PG |
| error | false | copper-clearance | net | CAN1_TXD |  |  | copper of "CAN1_TXD" and "PMIC_MAIN_12V0" closer than 0.127mm at 1 place(s); worst gap -0.250mm near (5.00, -78.00)mm |  |  | neighbour=PMIC_MAIN_12V0 |

## Skipped rules

6 rows

| rule | reason |
|---|---|
| unconnected-pin | source format cannot express intentional no-connect |
| wire-no-junction | this format's reader does not examine wire geometry, so no T-tap was checked |
| duplicate-ref-des | this format's reader does not detect ref-des collisions (nothing was checked) |
| power-input-not-driven | source format does not type power-output pins (driver absence is not conclusive here) |
| netclass-track-width | design declares no net-class definitions, so there is no declared limit to compare against |

## Verdicts

273 rows

| verdict_id | url | rule | outcome | subjects | statement | context | terms | reason |
|---|---|---|---|---|---|---|---|---|
| annular-width:(net:CAN1_CANH) |  | annular-width | pass | net:CAN1_CANH | passes because thin is 0, not >= 1 |  |  |  |
| annular-width:(net:CAN1_CANL) |  | annular-width | pass | net:CAN1_CANL | passes because thin is 0, not >= 1 |  |  |  |
| annular-width:(net:CAN1_RXD) |  | annular-width | pass | net:CAN1_RXD | passes because thin is 0, not >= 1 |  |  |  |
| annular-width:(net:CAN1_TXD) |  | annular-width | pass | net:CAN1_TXD | passes because thin is 0, not >= 1 |  |  |  |
| annular-width:(net:CLK_IN) |  | annular-width | pass | net:CLK_IN | passes because thin is 0, not >= 1 |  |  |  |

## Verdicts by rule

38 rows

| rule | pass | fail | inconclusive | no-limit | not-considered | total |
|---|---|---|---|---|---|---|
| annular-width | 15 | 0 | 0 | 0 | 0 | 15 |
| copper-clearance | 0 | 12 | 0 | 0 | 0 | 12 |
| crystal-load-caps | 2 | 0 | 0 | 0 | 0 | 2 |
| decoupling-present | 2 | 1 | 0 | 0 | 0 | 3 |
| duplicate-net-name | 15 | 0 | 0 | 0 | 0 | 15 |
