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
    __slots__ = ("net", "pin")
    class ClassEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: ClassVocab
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[ClassVocab, _Mapping]] = ...) -> None: ...
    NET_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    CLASS_FIELD_NUMBER: _ClassVar[int]
    net: NetNameVocab
    pin: PinNameVocab
    def __init__(self, net: _Optional[_Union[NetNameVocab, _Mapping]] = ..., pin: _Optional[_Union[PinNameVocab, _Mapping]] = ..., **kwargs) -> None: ...

class NetNameVocab(_message.Message):
    __slots__ = ("rail", "ground", "feedback", "switching", "control", "gate_drive")
    RAIL_FIELD_NUMBER: _ClassVar[int]
    GROUND_FIELD_NUMBER: _ClassVar[int]
    FEEDBACK_FIELD_NUMBER: _ClassVar[int]
    SWITCHING_FIELD_NUMBER: _ClassVar[int]
    CONTROL_FIELD_NUMBER: _ClassVar[int]
    GATE_DRIVE_FIELD_NUMBER: _ClassVar[int]
    rail: VocabPatterns
    ground: VocabPatterns
    feedback: VocabPatterns
    switching: VocabPatterns
    control: VocabPatterns
    gate_drive: VocabPatterns
    def __init__(self, rail: _Optional[_Union[VocabPatterns, _Mapping]] = ..., ground: _Optional[_Union[VocabPatterns, _Mapping]] = ..., feedback: _Optional[_Union[VocabPatterns, _Mapping]] = ..., switching: _Optional[_Union[VocabPatterns, _Mapping]] = ..., control: _Optional[_Union[VocabPatterns, _Mapping]] = ..., gate_drive: _Optional[_Union[VocabPatterns, _Mapping]] = ...) -> None: ...

class PinNameVocab(_message.Message):
    __slots__ = ("supply", "gate", "source", "drain")
    SUPPLY_FIELD_NUMBER: _ClassVar[int]
    GATE_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    DRAIN_FIELD_NUMBER: _ClassVar[int]
    supply: VocabPatterns
    gate: VocabPatterns
    source: VocabPatterns
    drain: VocabPatterns
    def __init__(self, supply: _Optional[_Union[VocabPatterns, _Mapping]] = ..., gate: _Optional[_Union[VocabPatterns, _Mapping]] = ..., source: _Optional[_Union[VocabPatterns, _Mapping]] = ..., drain: _Optional[_Union[VocabPatterns, _Mapping]] = ...) -> None: ...

class VocabPatterns(_message.Message):
    __slots__ = ("patterns", "replace")
    PATTERNS_FIELD_NUMBER: _ClassVar[int]
    REPLACE_FIELD_NUMBER: _ClassVar[int]
    patterns: _containers.RepeatedScalarFieldContainer[str]
    replace: bool
    def __init__(self, patterns: _Optional[_Iterable[str]] = ..., replace: _Optional[bool] = ...) -> None: ...

class ClassVocab(_message.Message):
    __slots__ = ("patterns", "replace", "prefixes")
    PATTERNS_FIELD_NUMBER: _ClassVar[int]
    REPLACE_FIELD_NUMBER: _ClassVar[int]
    PREFIXES_FIELD_NUMBER: _ClassVar[int]
    patterns: _containers.RepeatedScalarFieldContainer[str]
    replace: bool
    prefixes: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, patterns: _Optional[_Iterable[str]] = ..., replace: _Optional[bool] = ..., prefixes: _Optional[_Iterable[str]] = ...) -> None: ...

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
