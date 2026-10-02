from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DesignIntent(_message.Message):
    __slots__ = ("modules", "nets", "sequences", "strap_groups", "io_map", "margin_factor")
    class NetsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: NetIntent
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[NetIntent, _Mapping]] = ...) -> None: ...
    MODULES_FIELD_NUMBER: _ClassVar[int]
    NETS_FIELD_NUMBER: _ClassVar[int]
    SEQUENCES_FIELD_NUMBER: _ClassVar[int]
    STRAP_GROUPS_FIELD_NUMBER: _ClassVar[int]
    IO_MAP_FIELD_NUMBER: _ClassVar[int]
    MARGIN_FACTOR_FIELD_NUMBER: _ClassVar[int]
    modules: _containers.RepeatedCompositeFieldContainer[IntentModule]
    nets: _containers.MessageMap[str, NetIntent]
    sequences: _containers.RepeatedCompositeFieldContainer[PowerSequence]
    strap_groups: _containers.RepeatedCompositeFieldContainer[StrapGroup]
    io_map: _containers.RepeatedCompositeFieldContainer[PinAssignment]
    margin_factor: float
    def __init__(self, modules: _Optional[_Iterable[_Union[IntentModule, _Mapping]]] = ..., nets: _Optional[_Mapping[str, NetIntent]] = ..., sequences: _Optional[_Iterable[_Union[PowerSequence, _Mapping]]] = ..., strap_groups: _Optional[_Iterable[_Union[StrapGroup, _Mapping]]] = ..., io_map: _Optional[_Iterable[_Union[PinAssignment, _Mapping]]] = ..., margin_factor: _Optional[float] = ...) -> None: ...

class IntentModule(_message.Message):
    __slots__ = ("name", "mpn", "count", "nets")
    NAME_FIELD_NUMBER: _ClassVar[int]
    CLASS_FIELD_NUMBER: _ClassVar[int]
    MPN_FIELD_NUMBER: _ClassVar[int]
    COUNT_FIELD_NUMBER: _ClassVar[int]
    NETS_FIELD_NUMBER: _ClassVar[int]
    name: str
    mpn: str
    count: int
    nets: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, name: _Optional[str] = ..., mpn: _Optional[str] = ..., count: _Optional[int] = ..., nets: _Optional[_Iterable[str]] = ..., **kwargs) -> None: ...

class NetIntent(_message.Message):
    __slots__ = ("nominal", "domain", "peak", "protect", "reset", "strap", "min_ohms", "max_ohms", "ac_coupled")
    NOMINAL_FIELD_NUMBER: _ClassVar[int]
    DOMAIN_FIELD_NUMBER: _ClassVar[int]
    PEAK_FIELD_NUMBER: _ClassVar[int]
    PROTECT_FIELD_NUMBER: _ClassVar[int]
    RESET_FIELD_NUMBER: _ClassVar[int]
    STRAP_FIELD_NUMBER: _ClassVar[int]
    MIN_OHMS_FIELD_NUMBER: _ClassVar[int]
    MAX_OHMS_FIELD_NUMBER: _ClassVar[int]
    AC_COUPLED_FIELD_NUMBER: _ClassVar[int]
    nominal: float
    domain: str
    peak: float
    protect: _containers.RepeatedScalarFieldContainer[str]
    reset: str
    strap: str
    min_ohms: float
    max_ohms: float
    ac_coupled: bool
    def __init__(self, nominal: _Optional[float] = ..., domain: _Optional[str] = ..., peak: _Optional[float] = ..., protect: _Optional[_Iterable[str]] = ..., reset: _Optional[str] = ..., strap: _Optional[str] = ..., min_ohms: _Optional[float] = ..., max_ohms: _Optional[float] = ..., ac_coupled: _Optional[bool] = ...) -> None: ...

class PowerSequence(_message.Message):
    __slots__ = ("name", "relation", "order")
    NAME_FIELD_NUMBER: _ClassVar[int]
    RELATION_FIELD_NUMBER: _ClassVar[int]
    ORDER_FIELD_NUMBER: _ClassVar[int]
    name: str
    relation: str
    order: _containers.RepeatedCompositeFieldContainer[SequenceStage]
    def __init__(self, name: _Optional[str] = ..., relation: _Optional[str] = ..., order: _Optional[_Iterable[_Union[SequenceStage, _Mapping]]] = ...) -> None: ...

class SequenceStage(_message.Message):
    __slots__ = ("rail", "good", "enable")
    RAIL_FIELD_NUMBER: _ClassVar[int]
    GOOD_FIELD_NUMBER: _ClassVar[int]
    ENABLE_FIELD_NUMBER: _ClassVar[int]
    rail: str
    good: str
    enable: str
    def __init__(self, rail: _Optional[str] = ..., good: _Optional[str] = ..., enable: _Optional[str] = ...) -> None: ...

class StrapGroup(_message.Message):
    __slots__ = ("name", "device", "nets", "value", "bus", "default")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    NETS_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    BUS_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_FIELD_NUMBER: _ClassVar[int]
    name: str
    device: str
    nets: _containers.RepeatedScalarFieldContainer[str]
    value: int
    bus: str
    default: str
    def __init__(self, name: _Optional[str] = ..., device: _Optional[str] = ..., nets: _Optional[_Iterable[str]] = ..., value: _Optional[int] = ..., bus: _Optional[str] = ..., default: _Optional[str] = ...) -> None: ...

class PinAssignment(_message.Message):
    __slots__ = ("net", "device", "pin", "function", "to")
    NET_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    FUNCTION_FIELD_NUMBER: _ClassVar[int]
    TO_FIELD_NUMBER: _ClassVar[int]
    net: str
    device: str
    pin: str
    function: str
    to: PinEndpoint
    def __init__(self, net: _Optional[str] = ..., device: _Optional[str] = ..., pin: _Optional[str] = ..., function: _Optional[str] = ..., to: _Optional[_Union[PinEndpoint, _Mapping]] = ...) -> None: ...

class PinEndpoint(_message.Message):
    __slots__ = ("device", "pin")
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    device: str
    pin: str
    def __init__(self, device: _Optional[str] = ..., pin: _Optional[str] = ...) -> None: ...
