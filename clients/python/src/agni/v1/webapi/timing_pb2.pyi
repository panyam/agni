from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RequestTiming(_message.Message):
    __slots__ = ("procedure", "total_micros", "stages", "lookups", "rules", "rules_evaluated", "plans")
    PROCEDURE_FIELD_NUMBER: _ClassVar[int]
    TOTAL_MICROS_FIELD_NUMBER: _ClassVar[int]
    STAGES_FIELD_NUMBER: _ClassVar[int]
    LOOKUPS_FIELD_NUMBER: _ClassVar[int]
    RULES_FIELD_NUMBER: _ClassVar[int]
    RULES_EVALUATED_FIELD_NUMBER: _ClassVar[int]
    PLANS_FIELD_NUMBER: _ClassVar[int]
    procedure: str
    total_micros: int
    stages: _containers.RepeatedCompositeFieldContainer[TimedStage]
    lookups: _containers.RepeatedCompositeFieldContainer[CacheLookup]
    rules: _containers.RepeatedCompositeFieldContainer[RuleTiming]
    rules_evaluated: int
    plans: _containers.RepeatedCompositeFieldContainer[QueryPlan]
    def __init__(self, procedure: _Optional[str] = ..., total_micros: _Optional[int] = ..., stages: _Optional[_Iterable[_Union[TimedStage, _Mapping]]] = ..., lookups: _Optional[_Iterable[_Union[CacheLookup, _Mapping]]] = ..., rules: _Optional[_Iterable[_Union[RuleTiming, _Mapping]]] = ..., rules_evaluated: _Optional[int] = ..., plans: _Optional[_Iterable[_Union[QueryPlan, _Mapping]]] = ...) -> None: ...

class TimedStage(_message.Message):
    __slots__ = ("name", "start_micros", "micros")
    NAME_FIELD_NUMBER: _ClassVar[int]
    START_MICROS_FIELD_NUMBER: _ClassVar[int]
    MICROS_FIELD_NUMBER: _ClassVar[int]
    name: str
    start_micros: int
    micros: int
    def __init__(self, name: _Optional[str] = ..., start_micros: _Optional[int] = ..., micros: _Optional[int] = ...) -> None: ...

class CacheLookup(_message.Message):
    __slots__ = ("layer", "source")
    LAYER_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    layer: str
    source: str
    def __init__(self, layer: _Optional[str] = ..., source: _Optional[str] = ...) -> None: ...

class RuleTiming(_message.Message):
    __slots__ = ("rule", "micros")
    RULE_FIELD_NUMBER: _ClassVar[int]
    MICROS_FIELD_NUMBER: _ClassVar[int]
    rule: str
    micros: int
    def __init__(self, rule: _Optional[str] = ..., micros: _Optional[int] = ...) -> None: ...

class QueryPlan(_message.Message):
    __slots__ = ("query", "plan")
    QUERY_FIELD_NUMBER: _ClassVar[int]
    PLAN_FIELD_NUMBER: _ClassVar[int]
    query: str
    plan: str
    def __init__(self, query: _Optional[str] = ..., plan: _Optional[str] = ...) -> None: ...
