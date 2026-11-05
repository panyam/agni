from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class NamingConvention(_message.Message):
    __slots__ = ("name", "lexicon", "rules")
    NAME_FIELD_NUMBER: _ClassVar[int]
    LEXICON_FIELD_NUMBER: _ClassVar[int]
    RULES_FIELD_NUMBER: _ClassVar[int]
    name: str
    lexicon: NamingLexicon
    rules: _containers.RepeatedCompositeFieldContainer[NamingRule]
    def __init__(self, name: _Optional[str] = ..., lexicon: _Optional[_Union[NamingLexicon, _Mapping]] = ..., rules: _Optional[_Iterable[_Union[NamingRule, _Mapping]]] = ...) -> None: ...

class NamingLexicon(_message.Message):
    __slots__ = ("net", "pin", "value_unit", "attribute")
    class ClassEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: ClassVocab
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[ClassVocab, _Mapping]] = ...) -> None: ...
    class ValueUnitEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NET_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    CLASS_FIELD_NUMBER: _ClassVar[int]
    VALUE_UNIT_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTE_FIELD_NUMBER: _ClassVar[int]
    net: NetNameVocab
    pin: PinNameVocab
    value_unit: _containers.ScalarMap[str, str]
    attribute: AttributeVocab
    def __init__(self, net: _Optional[_Union[NetNameVocab, _Mapping]] = ..., pin: _Optional[_Union[PinNameVocab, _Mapping]] = ..., value_unit: _Optional[_Mapping[str, str]] = ..., attribute: _Optional[_Union[AttributeVocab, _Mapping]] = ..., **kwargs) -> None: ...

class AttributeVocab(_message.Message):
    __slots__ = ("mpn", "value", "dnp", "dnp_values")
    MPN_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    DNP_FIELD_NUMBER: _ClassVar[int]
    DNP_VALUES_FIELD_NUMBER: _ClassVar[int]
    mpn: AttributeKeys
    value: AttributeKeys
    dnp: AttributeKeys
    dnp_values: AttributeKeys
    def __init__(self, mpn: _Optional[_Union[AttributeKeys, _Mapping]] = ..., value: _Optional[_Union[AttributeKeys, _Mapping]] = ..., dnp: _Optional[_Union[AttributeKeys, _Mapping]] = ..., dnp_values: _Optional[_Union[AttributeKeys, _Mapping]] = ...) -> None: ...

class AttributeKeys(_message.Message):
    __slots__ = ("keys", "replace")
    KEYS_FIELD_NUMBER: _ClassVar[int]
    REPLACE_FIELD_NUMBER: _ClassVar[int]
    keys: _containers.RepeatedScalarFieldContainer[str]
    replace: bool
    def __init__(self, keys: _Optional[_Iterable[str]] = ..., replace: _Optional[bool] = ...) -> None: ...

class NetNameVocab(_message.Message):
    __slots__ = ("rail", "ground", "feedback", "switching", "control", "gate_drive", "i2c", "infer_rail_from_supply_pins", "no_connect", "diff_pair", "voltage")
    RAIL_FIELD_NUMBER: _ClassVar[int]
    GROUND_FIELD_NUMBER: _ClassVar[int]
    FEEDBACK_FIELD_NUMBER: _ClassVar[int]
    SWITCHING_FIELD_NUMBER: _ClassVar[int]
    CONTROL_FIELD_NUMBER: _ClassVar[int]
    GATE_DRIVE_FIELD_NUMBER: _ClassVar[int]
    I2C_FIELD_NUMBER: _ClassVar[int]
    INFER_RAIL_FROM_SUPPLY_PINS_FIELD_NUMBER: _ClassVar[int]
    NO_CONNECT_FIELD_NUMBER: _ClassVar[int]
    DIFF_PAIR_FIELD_NUMBER: _ClassVar[int]
    VOLTAGE_FIELD_NUMBER: _ClassVar[int]
    rail: VocabPatterns
    ground: VocabPatterns
    feedback: VocabPatterns
    switching: VocabPatterns
    control: VocabPatterns
    gate_drive: VocabPatterns
    i2c: VocabPatterns
    infer_rail_from_supply_pins: bool
    no_connect: VocabPatterns
    diff_pair: DiffPairVocab
    voltage: VocabPatterns
    def __init__(self, rail: _Optional[_Union[VocabPatterns, _Mapping]] = ..., ground: _Optional[_Union[VocabPatterns, _Mapping]] = ..., feedback: _Optional[_Union[VocabPatterns, _Mapping]] = ..., switching: _Optional[_Union[VocabPatterns, _Mapping]] = ..., control: _Optional[_Union[VocabPatterns, _Mapping]] = ..., gate_drive: _Optional[_Union[VocabPatterns, _Mapping]] = ..., i2c: _Optional[_Union[VocabPatterns, _Mapping]] = ..., infer_rail_from_supply_pins: _Optional[bool] = ..., no_connect: _Optional[_Union[VocabPatterns, _Mapping]] = ..., diff_pair: _Optional[_Union[DiffPairVocab, _Mapping]] = ..., voltage: _Optional[_Union[VocabPatterns, _Mapping]] = ...) -> None: ...

class DiffPairVocab(_message.Message):
    __slots__ = ("pairs", "replace")
    PAIRS_FIELD_NUMBER: _ClassVar[int]
    REPLACE_FIELD_NUMBER: _ClassVar[int]
    pairs: _containers.RepeatedCompositeFieldContainer[DiffSuffix]
    replace: bool
    def __init__(self, pairs: _Optional[_Iterable[_Union[DiffSuffix, _Mapping]]] = ..., replace: _Optional[bool] = ...) -> None: ...

class DiffSuffix(_message.Message):
    __slots__ = ("positive", "negative")
    POSITIVE_FIELD_NUMBER: _ClassVar[int]
    NEGATIVE_FIELD_NUMBER: _ClassVar[int]
    positive: str
    negative: str
    def __init__(self, positive: _Optional[str] = ..., negative: _Optional[str] = ...) -> None: ...

class PinNameVocab(_message.Message):
    __slots__ = ("supply", "gate", "source", "drain", "exposed_pad", "anode", "cathode", "feedback", "switching", "through_pair")
    SUPPLY_FIELD_NUMBER: _ClassVar[int]
    GATE_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    DRAIN_FIELD_NUMBER: _ClassVar[int]
    EXPOSED_PAD_FIELD_NUMBER: _ClassVar[int]
    ANODE_FIELD_NUMBER: _ClassVar[int]
    CATHODE_FIELD_NUMBER: _ClassVar[int]
    FEEDBACK_FIELD_NUMBER: _ClassVar[int]
    SWITCHING_FIELD_NUMBER: _ClassVar[int]
    THROUGH_PAIR_FIELD_NUMBER: _ClassVar[int]
    supply: VocabPatterns
    gate: VocabPatterns
    source: VocabPatterns
    drain: VocabPatterns
    exposed_pad: VocabPatterns
    anode: VocabPatterns
    cathode: VocabPatterns
    feedback: VocabPatterns
    switching: VocabPatterns
    through_pair: PinPairVocab
    def __init__(self, supply: _Optional[_Union[VocabPatterns, _Mapping]] = ..., gate: _Optional[_Union[VocabPatterns, _Mapping]] = ..., source: _Optional[_Union[VocabPatterns, _Mapping]] = ..., drain: _Optional[_Union[VocabPatterns, _Mapping]] = ..., exposed_pad: _Optional[_Union[VocabPatterns, _Mapping]] = ..., anode: _Optional[_Union[VocabPatterns, _Mapping]] = ..., cathode: _Optional[_Union[VocabPatterns, _Mapping]] = ..., feedback: _Optional[_Union[VocabPatterns, _Mapping]] = ..., switching: _Optional[_Union[VocabPatterns, _Mapping]] = ..., through_pair: _Optional[_Union[PinPairVocab, _Mapping]] = ...) -> None: ...

class PinPairVocab(_message.Message):
    __slots__ = ("pairs", "replace")
    PAIRS_FIELD_NUMBER: _ClassVar[int]
    REPLACE_FIELD_NUMBER: _ClassVar[int]
    pairs: _containers.RepeatedCompositeFieldContainer[PinPair]
    replace: bool
    def __init__(self, pairs: _Optional[_Iterable[_Union[PinPair, _Mapping]]] = ..., replace: _Optional[bool] = ...) -> None: ...

class PinPair(_message.Message):
    __slots__ = ("a", "b")
    A_FIELD_NUMBER: _ClassVar[int]
    B_FIELD_NUMBER: _ClassVar[int]
    a: str
    b: str
    def __init__(self, a: _Optional[str] = ..., b: _Optional[str] = ...) -> None: ...

class VocabPatterns(_message.Message):
    __slots__ = ("patterns", "replace", "exclude")
    PATTERNS_FIELD_NUMBER: _ClassVar[int]
    REPLACE_FIELD_NUMBER: _ClassVar[int]
    EXCLUDE_FIELD_NUMBER: _ClassVar[int]
    patterns: _containers.RepeatedScalarFieldContainer[str]
    replace: bool
    exclude: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, patterns: _Optional[_Iterable[str]] = ..., replace: _Optional[bool] = ..., exclude: _Optional[_Iterable[str]] = ...) -> None: ...

class ClassVocab(_message.Message):
    __slots__ = ("patterns", "replace", "prefixes", "families", "exclude_prefixes")
    PATTERNS_FIELD_NUMBER: _ClassVar[int]
    REPLACE_FIELD_NUMBER: _ClassVar[int]
    PREFIXES_FIELD_NUMBER: _ClassVar[int]
    FAMILIES_FIELD_NUMBER: _ClassVar[int]
    EXCLUDE_PREFIXES_FIELD_NUMBER: _ClassVar[int]
    patterns: _containers.RepeatedScalarFieldContainer[str]
    replace: bool
    prefixes: _containers.RepeatedScalarFieldContainer[str]
    families: _containers.RepeatedScalarFieldContainer[str]
    exclude_prefixes: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, patterns: _Optional[_Iterable[str]] = ..., replace: _Optional[bool] = ..., prefixes: _Optional[_Iterable[str]] = ..., families: _Optional[_Iterable[str]] = ..., exclude_prefixes: _Optional[_Iterable[str]] = ...) -> None: ...

class NamingRule(_message.Message):
    __slots__ = ("name", "severity", "why", "allow", "exempt", "match_full")
    NAME_FIELD_NUMBER: _ClassVar[int]
    SEVERITY_FIELD_NUMBER: _ClassVar[int]
    WHY_FIELD_NUMBER: _ClassVar[int]
    ALLOW_FIELD_NUMBER: _ClassVar[int]
    EXEMPT_FIELD_NUMBER: _ClassVar[int]
    MATCH_FULL_FIELD_NUMBER: _ClassVar[int]
    name: str
    severity: str
    why: str
    allow: _containers.RepeatedScalarFieldContainer[str]
    exempt: _containers.RepeatedScalarFieldContainer[str]
    match_full: bool
    def __init__(self, name: _Optional[str] = ..., severity: _Optional[str] = ..., why: _Optional[str] = ..., allow: _Optional[_Iterable[str]] = ..., exempt: _Optional[_Iterable[str]] = ..., match_full: _Optional[bool] = ...) -> None: ...
